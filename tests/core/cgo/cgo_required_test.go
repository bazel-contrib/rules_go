//go:build cgo
// +build cgo

package cgo_required

import "testing"

// Without correct filtering, gentestmain will try to link against this test
// that does not exist with pure = True
func TestHelloWorld(t *testing.T) {}
