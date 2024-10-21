package sysod

import(
	"testing"
)

func foo() int32

func TestSysoDirect(t *testing.T) {
	want := int32(42)
	got := foo()
	if want != got {
		t.Errorf("want: %d, got %d", want, got)
	}
}
