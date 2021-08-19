package main

import (
	"fmt"
	"interface/progress"
	"interface/res"
	"io/ioutil"
	"math/rand"
	"os"
	"time"

	"github.com/BurntSushi/toml"
	"github.com/spf13/cobra"
)

var conf Config

var config string
var output string

var rootCmd = &cobra.Command{
	Use:   "sync-diff-inspector",
	Short: "SyncDiff",
	Long:  `Sync Diff Inspector`,
	RunE: func(cmd *cobra.Command, args []string) error {
		confB, err := ioutil.ReadFile(config)
		if err != nil {
			return err
		}
		_, err = toml.Decode(string(confB), &conf)
		if err != nil {
			return err
		}
		p := progress.NewTableProgressPrinter(5)
		rand.Seed(time.Now().UnixNano())
		for i := 0; i < 5; i++ {
			name := fmt.Sprintf("schema%d.table%d", i, i)
			p.StartTable(name, 50, false, true)
			for j := 0; j < 50; j++ {
				if i == 3 && j == 40 {
					p.FailTable(name)
					break
				}
				time.Sleep(time.Duration(50 * time.Millisecond))
				p.Inc(name)
			}
			time.Sleep(time.Duration(500 * time.Millisecond))
		}
		p.Close()
		p.PrintSummary(conf.Task.OutputDir)
		return writeOutput(conf.Task.OutputDir)
	},
}

var generateCmd = &cobra.Command{
	Use:   "generate-config",
	Short: "Sync Diff Inspector config file generater",
	Long:  `Sync Diff Inspector config file generater`,
	Run:   func(cmd *cobra.Command, args []string) {},
}

var templateCmd = &cobra.Command{
	Use:   "template",
	Short: "generate a config file template",
	Long:  `generate a config file template`,
	RunE: func(cmd *cobra.Command, args []string) error {
		err := ioutil.WriteFile(output, res.CONFIG, 0644)
		if err != nil {
			return err
		}
		println("output a config file template for sync-diff-inspector to: " + output)
		return nil
	},
}

func init() {
	rootCmd.SetOut(os.Stdout)
	rootCmd.CompletionOptions.DisableDefaultCmd = true
	rootCmd.Flags().StringVarP(&config, "config", "c", "", "Config file for sync diff inspector")
	templateCmd.Flags().StringVarP(&output, "output", "o", "sync-diff-template.toml", "Config file output patch")
	generateCmd.AddCommand(templateCmd)
	rootCmd.AddCommand(generateCmd)
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		rootCmd.Println(err)
		os.Exit(1)
	}
}

func writeOutput(dir string) error {
	err := os.MkdirAll(dir, 0755)
	if err != nil {
		return err
	}
	err = os.MkdirAll(dir+"/patch", 0755)
	if err != nil {
		return err
	}
	err = os.MkdirAll(dir+"/checkpoint", 0755)
	if err != nil {
		return err
	}
	err = ioutil.WriteFile(dir+"/summary.txt", res.SUMMARY, 0644)
	if err != nil {
		return err
	}

	err = ioutil.WriteFile(dir+"/sync-diff-inspector.log", res.LOG, 0644)
	if err != nil {
		return err
	}

	err = ioutil.WriteFile(dir+"/checkpoint/README.txt", res.CHECKPOINT_README, 0644)
	if err != nil {
		return err
	}

	err = ioutil.WriteFile(dir+"/patch/patch-target-tidb1-1.sql", res.PATCH1, 0644)
	if err != nil {
		return err
	}

	err = ioutil.WriteFile(dir+"/patch/patch-target-tidb1-2.sql", res.PATCH2, 0644)
	if err != nil {
		return err
	}
	return nil

}
