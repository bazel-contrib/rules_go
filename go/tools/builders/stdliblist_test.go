package main

import (
	"encoding/json"
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

func Test_stdliblist_noexport(t *testing.T) {
	testDir := t.TempDir()
	outJSON := filepath.Join(testDir, "out.json")

	test_args := []string{
		"-out=" + outJSON,
		"-sdk=../" + sdkDir(),
	}

	if err := stdliblist(test_args); err != nil {
		t.Errorf("calling stdliblist got err: %v", err)
	}
	f, err := os.Open(outJSON)
	if err != nil {
		t.Errorf("cannot open output json: %v", err)
	}
	defer func() { _ = f.Close() }()
	decoder := json.NewDecoder(f)
	for decoder.More() {
		var result *flatPackage
		if err := decoder.Decode(&result); err != nil {
			t.Errorf("unable to decode output json: %v\n", err)
		}

		if !strings.HasPrefix(result.ID, "@@//stdlib:") {
			t.Errorf("ID should be prefixed with @@//stdlib: :%v", result)
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

	if err := stdliblist(test_args); err != nil {
		t.Errorf("calling stdliblist got err: %v", err)
	}
	f, err := os.Open(outJSON)
	if err != nil {
		t.Errorf("cannot open output json: %v", err)
	}
	defer func() { _ = f.Close() }()
	decoder := json.NewDecoder(f)
	anyExportSet := false
	for decoder.More() {
		var result *flatPackage
		if err := decoder.Decode(&result); err != nil {
			t.Errorf("unable to decode output json: %v\n", err)
		}

		if !strings.HasPrefix(result.ID, "@@//stdlib:") {
			t.Errorf("ID should be prefixed with @@//stdlib: :%v", result)
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

func Test_absoluteSourcesPaths(t *testing.T) {
	cloneBase := filepath.FromSlash("/work/clone")
	cache := filepath.FromSlash("/work/cache")
	pathReplaceFn := func(p string) string {
		if strings.HasPrefix(p, cache) {
			return strings.Replace(p, cache, "__BAZEL_EXECROOT__", 1)
		}
		return p
	}

	for _, tc := range []struct {
		desc, dir, want string
	}{
		{
			desc: "goroot",
			dir:  "/work/clone/external/go_sdk/src/crypto/sha256",
			want: "__BAZEL_OUTPUT_BASE__/external/go_sdk/src/crypto/sha256/sha256.go",
		},
		{
			desc: "fips_snapshot",
			dir:  "/work/cache/golang.org/fips140@v1.0.0/fips140/v1.0.0/sha256",
			want: "__BAZEL_EXECROOT__/golang.org/fips140@v1.0.0/fips140/v1.0.0/sha256/sha256.go",
		},
	} {
		t.Run(tc.desc, func(t *testing.T) {
			got := absoluteSourcesPaths(cloneBase, filepath.FromSlash(tc.dir), []string{"sha256.go"}, pathReplaceFn)
			if want := filepath.FromSlash(tc.want); len(got) != 1 || got[0] != want {
				t.Errorf("got %v, want [%s]", got, want)
			}
		})
	}
}
