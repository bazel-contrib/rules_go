package main

import (
	"github.com/bazelbuild/rules_go/go/tools/bzltestutil/bincov"
)

func init() {
	bincov.AddExitHook()
}
