package objc

import (
	"fmt"
	"math/rand"
	"testing"
)

func TestCPPAdder(t *testing.T) {
	a := int32(rand.Intn(100))
	b := int32(rand.Intn(100))
	expected := a + b
	if result := AddC(a, b); result != expected {
		t.Error(fmt.Errorf("wrong result: got %d, expected %d", result, expected))
	}
	if result := AddCPP(a, b); result != expected {
		t.Error(fmt.Errorf("wrong result: got %d, expected %d", result, expected))
	}
}
