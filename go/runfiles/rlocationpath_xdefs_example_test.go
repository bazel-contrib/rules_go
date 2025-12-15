package runfiles_test

import (
	"os"

	"github.com/bazelbuild/rules_go/go/runfiles"
)

// Injected by Bazel by adding the following attribute on the target
// (supported by go_binary, go_test, and go_library):
//
// x_defs = {
//     "example.com/pkg.fileTxtRlocationpath": "$(rlocationpath file.txt)",
// },
var fileTxtRlocationpath string

// Read a runfile at a path injected by Bazel at build time.
func ExampleRlocation_injectedPath() {
	p, err := runfiles.Rlocation(fileTxtRlocationpath)
	if err != nil {
		panic(err)
	}
	content, _ := os.ReadFile(p)
	println(string(content))
}
