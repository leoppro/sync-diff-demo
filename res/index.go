package res

import _ "embed"

//go:embed config.toml
var CONFIG []byte

//go:embed summary.txt
var SUMMARY []byte

//go:embed sync-diff-inspector.log
var LOG []byte

//go:embed patch-target-tidb1-1.sql
var PATCH1 []byte

//go:embed patch-target-tidb1-2.sql
var PATCH2 []byte

//go:embed checkpoint_README.txt
var CHECKPOINT_README []byte
