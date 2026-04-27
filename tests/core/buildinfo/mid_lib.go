package mid

import "github.com/bazelbuild/rules_go/tests/core/buildinfo/leaf"

// MidFunc calls the leaf library
func MidFunc() string {
	return "mid-" + leaf.LeafFunc()
}
