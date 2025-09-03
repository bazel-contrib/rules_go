package ccstaticinit

// #include "tests/core/cgo/cc_static_init/lib.h"
import "C"
import "testing"

func TestValue(t *testing.T) {
	const expected = 42
	actual := int(*C.GetValue())
	if expected != actual {
		t.Errorf("Expected %v, got %v", expected, actual)
	}
}
