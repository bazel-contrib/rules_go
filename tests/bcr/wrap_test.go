package main

import (
	"runtime"
	"strings"
	"testing"
)

func TestSdkVersion(t *testing.T) {
	if !strings.Contains(runtime.Version(), "1.23.6") {
		t.Fatal("Incorrect toolchain version", runtime.Version())
	}
}
