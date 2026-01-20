package cdeps_embed

import "testing"

// TestCdepsEmbedPropagation tests that cdeps are properly propagated
// when a go_library with cdeps embeds another go_library with cgo=True,
// and a go_test embeds the library with cdeps.
//
// This is a regression test for the issue where cdeps were not being
// propagated through embed chains in rules_go v0.51.0+.
func TestCdepsEmbedPropagation(t *testing.T) {
	// If cdeps are properly propagated, this will link successfully
	// and call the C function from native_dep.
	CallNative()
}
