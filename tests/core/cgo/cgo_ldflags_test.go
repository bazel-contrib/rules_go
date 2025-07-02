package main

// void bar();
import "C"

import (
	"testing"
)

// String value to be populated via `-X` linker flag.
var (
	value1 string
)

func TestInjectedLinkerValues(t *testing.T) {
	expectedValue1 := "314"

	if value1 == "" {
		t.Errorf("`value1` was not set by the linker flag. Expected non-empty string.")
	}

	if value1 != expectedValue1 {
		t.Errorf("Incorrect value for `value1`: got %s, want %s", value1, expectedValue1)
	}

}

func TestBar(t *testing.T) {
	C.bar()
}
