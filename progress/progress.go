package progress

import (
	"container/list"
	"fmt"
	"io"
	"os"
	"strings"
	"time"
)

type TableProgressPrinter struct {
	tableList      *list.List
	stateIsChanged bool
	tableFailList  *list.List
	tableMap       map[string]*list.Element
	output         io.Writer
	lines          int

	finishTableNums int
	tableNums       int

	progress int
	total    int

	optCh    chan Operator
	finishCh chan struct{}
}

type table_state_t int

const (
	TABLE_STATE_FAIL_STRUCTURE table_state_t = iota
	TABLE_STATE_PRESTART
	TABLE_STATE_COMPARING
	TABLE_STATE_SAME
	TABLE_STATE_DIFFERENT
	TABLE_STATE_HEAD
)

type TableProgress struct {
	name            string
	progress        int
	total           int
	state           table_state_t
	totalStopUpdate bool
}

type progress_opt_t int

const (
	PROGRESS_OPT_INC progress_opt_t = iota
	PROGRESS_OPT_UPDATE
	PROGRESS_OPT_START
	PROGRESS_OPT_FAIL
	PROGRESS_OPT_CLOSE
	PROGRESS_OPT_ERROR
)

type Operator struct {
	optType         progress_opt_t
	name            string
	total           int
	state           table_state_t
	totalStopUpdate bool
}

func NewTableProgressPrinter(tableNums int) *TableProgressPrinter {
	tpp := &TableProgressPrinter{
		tableList:       list.New(),
		stateIsChanged:  false,
		tableFailList:   list.New(),
		tableMap:        make(map[string]*list.Element),
		output:          os.Stdout,
		lines:           0,
		finishTableNums: 0,
		tableNums:       tableNums,

		progress: 0,
		total:    0,

		optCh:    make(chan Operator, 16),
		finishCh: make(chan struct{}),
	}
	tpp.init()
	go tpp.serve()
	fmt.Fprintf(tpp.output, "A total of %d tables need to be compared\n\n\n", tableNums)
	return tpp
}

func (tpp *TableProgressPrinter) Inc(name string) {
	tpp.optCh <- Operator{
		optType: PROGRESS_OPT_INC,
		name:    name,
	}
}

func (tpp *TableProgressPrinter) UpdateTotal(name string, total int, stopUpdate bool) {
	tpp.optCh <- Operator{
		optType:         PROGRESS_OPT_UPDATE,
		name:            name,
		total:           total,
		totalStopUpdate: stopUpdate,
	}
}

func (tpp *TableProgressPrinter) StartTable(name string, total int, isFailed bool, stopUpdate bool) {
	var state table_state_t
	if isFailed {
		state = TABLE_STATE_FAIL_STRUCTURE
	} else {
		state = TABLE_STATE_PRESTART
	}
	tpp.optCh <- Operator{
		optType:         PROGRESS_OPT_START,
		name:            name,
		total:           total,
		state:           state,
		totalStopUpdate: stopUpdate,
	}
}

func (tpp *TableProgressPrinter) FailTable(name string) {
	tpp.optCh <- Operator{
		optType: PROGRESS_OPT_FAIL,
		name:    name,
		state:   TABLE_STATE_DIFFERENT,
	}
}

func (tpp *TableProgressPrinter) Close() {
	tpp.optCh <- Operator{
		optType: PROGRESS_OPT_CLOSE,
	}
	<-tpp.finishCh
}

func (tpp *TableProgressPrinter) PrintSummary(output string) {
	var cleanStr, fixStr string
	cleanStr = "\x1b[1A\x1b[J"
	fixStr = "\nSummary:\n\n"
	if tpp.tableFailList.Len() == 0 {
		fixStr = fmt.Sprintf(
			"%sA total of %d tables have been compared and all are equal.\nYou can view the comparison summary and details through '%s'",
			fixStr,
			tpp.tableNums,
			output,
		)
	} else {
		fixStr = fmt.Sprintf(
			`%s
+----------------+--------------------+----------------+
| Table          | Structure equality | Data diff rows |
+----------------+--------------------+----------------+
| schema3.table3 | true               | +6/-2          |
+----------------+--------------------+----------------+
`, fixStr,
		)
		fixStr = fmt.Sprintf(
			"%s\nThe rest of the tables are all equal.\nThe patch file has been generated to '%s/patch'\nYou can view the comparison summary and details through '%s'\n",
			fixStr,
			output,
			output,
		)
	}

	fmt.Fprintf(tpp.output, "%s%s\n", cleanStr, fixStr)
	fmt.Fprintf(tpp.output, "Time Cost: 50s\nAverage speed: 34MB/s\n")

}

func (tpp *TableProgressPrinter) Error(err error) {
	tpp.optCh <- Operator{
		optType: PROGRESS_OPT_ERROR,
	}
	<-tpp.finishCh
	var cleanStr, fixStr string
	cleanStr = "\x1b[1A\x1b[J"
	fixStr = fmt.Sprintf("\nError in comparison process:\n%v\n\nYou can view the comparison details through './output_dir/sync_diff_inspector.log'\n", err)
	fmt.Fprintf(tpp.output, "%s%s", cleanStr, fixStr)
}

func (tpp *TableProgressPrinter) init() {
	tpp.tableList.PushBack(&TableProgress{
		state: TABLE_STATE_HEAD,
	})
}

func (tpp *TableProgressPrinter) serve() {
	tick := time.NewTicker(200 * time.Millisecond)

	for {
		select {
		case <-tick.C:
			tpp.flush()
		case opt := <-tpp.optCh:
			switch opt.optType {
			case PROGRESS_OPT_CLOSE:
				tpp.flush()
				tpp.finishCh <- struct{}{}
				return
			case PROGRESS_OPT_ERROR:
				tpp.finishCh <- struct{}{}
				return
			case PROGRESS_OPT_INC:
				if e, ok := tpp.tableMap[opt.name]; ok {
					tp := e.Value.(*TableProgress)
					tp.progress++
					tpp.progress++
					if tp.progress >= tp.total && tp.totalStopUpdate {
						tp.state = TABLE_STATE_SAME
						tpp.progress -= tp.progress
						tpp.total -= tp.total
						tpp.stateIsChanged = true
						delete(tpp.tableMap, opt.name)
					}
				}
			case PROGRESS_OPT_START:
				if _, ok := tpp.tableMap[opt.name]; !ok {
					e := tpp.tableList.PushBack(&TableProgress{
						name:            opt.name,
						progress:        0,
						total:           opt.total,
						state:           opt.state,
						totalStopUpdate: opt.totalStopUpdate,
					})
					tpp.tableMap[opt.name] = e
					if opt.state != TABLE_STATE_FAIL_STRUCTURE {
						tpp.total += opt.total
					}
					tpp.stateIsChanged = true
				}
			case PROGRESS_OPT_UPDATE:
				if e, ok := tpp.tableMap[opt.name]; ok {
					tp := e.Value.(*TableProgress)
					tpp.total += opt.total - tp.total
					tp.total = opt.total
					tp.totalStopUpdate = opt.totalStopUpdate
				}
			case PROGRESS_OPT_FAIL:
				if e, ok := tpp.tableMap[opt.name]; ok {
					tp := e.Value.(*TableProgress)
					tp.state = opt.state
					tpp.total -= tp.total
					tpp.progress -= tp.progress
					delete(tpp.tableMap, opt.name)
				}
			}
		}
	}
}

func (tpp *TableProgressPrinter) flush() {
	/*
	 * A total of 15 tables need to be compared
	 *
	 * Comparing the table structure of `schema1.table1` ... equivalent
	 * Comparing the table data of `schema1.table1` ... equivalent
	 * Comparing the table structure of `schema2.table2` ... equivalent
	 * Comparing the table data of `schema2.table2` ...
	 * _____________________________________________________________________________
	 * Progress [===================>-----------------------------------------] 35%
	 *
	 */

	if tpp.stateIsChanged {
		var cleanStr, fixStr, dynStr string
		cleanStr = fmt.Sprintf("\x1b[%dA\x1b[J", tpp.lines)
		tpp.lines = 2
		/* FAIL/PRESTART .... COMPARING/SAME/DIFFERENT */
		for p := tpp.tableList.Front(); p != nil; p = p.Next() {
			tp := p.Value.(*TableProgress)
			switch tp.state {
			case TABLE_STATE_PRESTART:
				fixStr = fmt.Sprintf("%sComparing the table structure of `%s` ... equivalent\n", fixStr, tp.name)
				dynStr = fmt.Sprintf("%sComparing the table data of `%s` ...\n", dynStr, tp.name)
				tpp.lines++
				tp.state = TABLE_STATE_COMPARING
			case TABLE_STATE_FAIL_STRUCTURE:
				fixStr = fmt.Sprintf("%sComparing the table structure of `%s` ... failure\n", fixStr, tp.name)
				tpp.tableFailList.PushBack(tp)
				// we have empty node as list head, so p is not nil
				preNode := p.Prev()
				tpp.tableList.Remove(p)
				p = preNode
			case TABLE_STATE_COMPARING:
				dynStr = fmt.Sprintf("%sComparing the table data of `%s` ...\n", dynStr, tp.name)
				tpp.lines++
			case TABLE_STATE_SAME:
				fixStr = fmt.Sprintf("%sComparing the table data of `%s` ... equivalent\n", fixStr, tp.name)
				// we have empty node as list head, so p is not nil
				preNode := p.Prev()
				tpp.tableList.Remove(p)
				p = preNode
				tpp.finishTableNums++
			case TABLE_STATE_DIFFERENT:
				fixStr = fmt.Sprintf("%sComparing the table data of `%s` ... failure\n", fixStr, tp.name)
				tpp.tableFailList.PushBack(tp)
				// we have empty node as list head, so p is not nil
				preNode := p.Prev()
				tpp.tableList.Remove(p)
				p = preNode
				tpp.finishTableNums++
			}
		}

		dynStr = fmt.Sprintf("%s_____________________________________________________________________________\n", dynStr)
		fmt.Fprintf(tpp.output, "%s%s%s", cleanStr, fixStr, dynStr)
		tpp.stateIsChanged = false
	} else {
		fmt.Fprint(tpp.output, "\x1b[1A\x1b[J")
	}
	// show bar
	// 60 '='+'-'
	leftTableNums := 1000 * (tpp.tableNums - tpp.finishTableNums)
	coe := float32(leftTableNums*tpp.progress)/float32(tpp.tableNums*(leftTableNums+1)*(tpp.total+1)) + float32(tpp.finishTableNums)/float32(tpp.tableNums)
	numLeft := int(60 * coe)
	percent := int(100 * coe)
	fmt.Fprintf(tpp.output, "Progress [%s>%s] %d%% %d/%d\n", strings.Repeat("=", numLeft), strings.Repeat("-", 60-numLeft), percent, tpp.progress, tpp.total)
}
