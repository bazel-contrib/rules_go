package main

import (
	"github.com/bazelbuild/rules_go/go/tools/bzltestutil"
)

func init() {
	bzltestutil.AddCoverageExitHook()
}
