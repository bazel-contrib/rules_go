//go:build amd64 || arm64

package asm_copts

import "testing"

func TestAssemblyCopts(t *testing.T) {
	expected := 99
	actual := CallAssemblyCopts()
	if actual != expected {
		t.Errorf("CallAssemblyCopts()=%d; expected=%d", actual, expected)
	}
}
