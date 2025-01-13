package main

import (
	"os"
	"strings"
	"testing"

	"github.com/bazelbuild/rules_go/go/runfiles"
)

func TestGenBoilerplate(t *testing.T) {
	version := "v1.2.3"
	shasum := "abcd1234"
	goVersion := "1.23"

	actual := genBoilerplate(version, shasum, goVersion)

	expectedPath, err := runfiles.Rlocation("_main/go/tools/releaser/testdata/boilerplate.md")
	if err != nil {
		t.Fatalf("failed to locate test data: %v", err)
	}
	expectedBytes, err := os.ReadFile(expectedPath)
	if err != nil {
		t.Fatalf("failed to read expected boilerplate: %v", err)
	}
	expected := strings.TrimSpace(string(expectedBytes))
	actual = strings.TrimSpace(actual)

	if actual != expected {
		t.Errorf("generated boilerplate does not match expected\nExpected:\n%s\nActual:\n%s", expected, actual)
	}
}
