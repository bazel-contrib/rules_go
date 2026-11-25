package main

import (
	"fmt"

	"github.com/bazelbuild/rules_go/tests/core/go_binary/stamp_override_lib"
)

func main() {
	fmt.Println(stamp_override_lib.GetVersion())
}
