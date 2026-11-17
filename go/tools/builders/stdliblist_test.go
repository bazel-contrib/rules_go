package main

import (
	"encoding/json"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"strings"
	"testing"
)

// Set via x_defs. The SDK's directory in the runfiles tree is its canonical
// repository name, which isn't known statically.
var goSDKRootFileRlocationPath string

func sdkDir() string {
	return path.Dir(goSDKRootFileRlocationPath)
}

// This test-only package exists in the SDK runfiles but is excluded from
// the standard library's declared sources.
const omittedPackage = "net/internal/socktest"

func sdkSourceArgs(t *testing.T) []string {
	t.Helper()
	src := filepath.Join("..", sdkDir(), "src")
	var args []string
	found := false
	if err := fs.WalkDir(os.DirFS(src), ".", func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() && p == omittedPackage {
			found = true
			return fs.SkipDir
		}
		if !d.IsDir() {
			args = append(args, "-src="+filepath.Join(src, p))
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if !found {
		t.Fatalf("SDK runfiles lack %q", omittedPackage)
	}
	return args
}

func Test_stdliblist_noexport(t *testing.T) {
	testDir := t.TempDir()
	outJSON := filepath.Join(testDir, "out.json")

	test_args := []string{
		"-out=" + outJSON,
		"-sdk=../" + sdkDir(),
	}

	if err := stdliblist(append(test_args, sdkSourceArgs(t)...)); err != nil {
		t.Fatal(err)
	}
	f, err := os.Open(outJSON)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = f.Close() }()
	decoder := json.NewDecoder(f)
	for decoder.More() {
		var result *flatPackage
		if err := decoder.Decode(&result); err != nil {
			t.Fatal(err)
		}
		if result.PkgPath == omittedPackage {
			t.Errorf("go list reported undeclared package %q", result.PkgPath)
		}

		if want := rulesGoStdlibPrefix + result.PkgPath; result.ID != want {
			t.Errorf("ID = %q, want %q", result.ID, want)
		}
		if result.ExportFile != "" {
			t.Errorf("ExportsFile should be empty when disabled but got: %v", result)
		}
		for _, gofile := range result.GoFiles {
			// The SDK runfiles are prefixed with __BAZEL_OUTPUT_BASE__/../<sdk>, which is cleaned.
			if !strings.HasPrefix(gofile, sdkDir()+"/") {
				t.Errorf("all go files should be prefixed with %s/ :%v", sdkDir(), result)
			}
		}
	}
}

func Test_stdliblist_export(t *testing.T) {
	testDir := t.TempDir()
	outJSON := filepath.Join(testDir, "out.json")
	test_args := []string{
		"-out=" + outJSON,
		"-sdk=../" + sdkDir(),
		"-export",
	}
	// Disable CGO otherwise, this takes forever to build.
	t.Setenv("CGO_ENABLED", "0")

	if err := stdliblist(append(test_args, sdkSourceArgs(t)...)); err != nil {
		t.Fatal(err)
	}
	f, err := os.Open(outJSON)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = f.Close() }()
	decoder := json.NewDecoder(f)
	anyExportSet := false
	for decoder.More() {
		var result *flatPackage
		if err := decoder.Decode(&result); err != nil {
			t.Fatal(err)
		}

		if want := rulesGoStdlibPrefix + result.PkgPath; result.ID != want {
			t.Errorf("ID = %q, want %q", result.ID, want)
		}
		if result.ExportFile != "" {
			anyExportSet = true
		}
		for _, gofile := range result.GoFiles {
			// The SDK runfiles are prefixed with __BAZEL_OUTPUT_BASE__/../<sdk>, which is cleaned.
			if !strings.HasPrefix(gofile, sdkDir()+"/") {
				t.Errorf("all go files should be prefixed with %s/ :%v", sdkDir(), result)
			}
		}
	}
	if !anyExportSet {
		t.Error("At least one export file should be set when -export is set.")
	}
}
