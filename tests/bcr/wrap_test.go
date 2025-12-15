package main

import (
	"runtime"
	"strings"
	"testing"
)

func TestSdkVersion(t *testing.T) {
	if !strings.Contains(runtime.Version(), "1.25.0") {
		t.Fatal("Incorrect toolchain version", runtime.Version())
	}
}
