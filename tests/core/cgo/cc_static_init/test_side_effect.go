package ccstaticinit

// #include "tests/core/cgo/cc_static_init/lib.h"
import "C"

import (
	"testing"
	"time"
)

func TestValue(t *testing.T) {
	// We could be too fast. Give the side_effect some time to complete
	time.Sleep(10 * time.Millisecond)
	const expected = 42
	actual := int(*C.GetValue())
	if expected != actual {
		t.Errorf("Expected %v, got %v", expected, actual)
	}
}
