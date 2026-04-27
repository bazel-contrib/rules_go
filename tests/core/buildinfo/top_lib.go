package top

import "github.com/bazelbuild/rules_go/tests/core/buildinfo/mid"

// TopFunc calls the mid library
func TopFunc() string {
	return "top-" + mid.MidFunc()
}
