package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestGenBoilerplate(t *testing.T) {
	version := "v1.2.3"
	shasum := "abcd1234"
	goVersion := "1.23"
	rnotesData := "Release notes"

	actual := genBoilerplate(version, shasum, goVersion, rnotesData)

	expectedPath := filepath.Join(".", "testdata", "boilerplate.md")
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
