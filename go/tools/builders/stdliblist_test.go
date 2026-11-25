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

func TestPruneUnreferencedCacheEntries(t *testing.T) {
	cacheDir := t.TempDir()
	files := map[string]bool{
		"README":         true,
		"aa/export-d":    true,
		"bb/generated-d": true,
		"aa/action-a":    false,
		"bb/unrelated-d": false,
		"cc/unrelated-d": false,
		"trim.txt":       false,
		"log.txt":        false,
	}
	referenced := map[string]bool{}
	for name, keep := range files {
		file := filepath.Join(cacheDir, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(file), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(file, []byte(name), 0o600); err != nil {
			t.Fatal(err)
		}
		if keep && name != "README" {
			referenced[file] = true
		}
	}

	// The action accepts a relative -cache path, but JSON references are absolute.
	t.Chdir(filepath.Dir(cacheDir))
	if err := pruneUnreferencedCacheEntries(filepath.Base(cacheDir), referenced); err != nil {
		t.Fatal(err)
	}
	for name, keep := range files {
		file := filepath.Join(cacheDir, filepath.FromSlash(name))
		contents, err := os.ReadFile(file)
		if keep {
			if err != nil || string(contents) != name {
				t.Errorf("referenced file %s changed or disappeared: %q, %v", name, contents, err)
			}
		} else if !os.IsNotExist(err) {
			t.Errorf("unreferenced file %s remains: %v", name, err)
		}
	}
	for _, shard := range []string{"aa", "bb", "cc"} {
		if info, err := os.Stat(filepath.Join(cacheDir, shard)); err != nil || !info.IsDir() {
			t.Errorf("cache shard %s disappeared: %v", shard, err)
		}
	}
}

func TestPruneUnreferencedCacheEntriesWithoutCache(t *testing.T) {
	t.Chdir(t.TempDir())
	const name = "unrelated.txt"
	if err := os.WriteFile(name, []byte("keep"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := pruneUnreferencedCacheEntries("", nil); err != nil {
		t.Fatal(err)
	}
	if contents, err := os.ReadFile(name); err != nil || string(contents) != "keep" {
		t.Fatalf("pruning without -cache changed the working directory: %q, %v", contents, err)
	}
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
	cacheDir := filepath.Join(testDir, "gocache")
	test_args := []string{
		"-out=" + outJSON,
		"-sdk=../" + sdkDir(),
		"-export",
		"-cache=" + cacheDir,
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
	referenced := map[string]bool{}
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
			cachePrefix := filepath.Join("__BAZEL_EXECROOT__", cacheDir) + string(filepath.Separator)
			if !strings.HasPrefix(result.ExportFile, cachePrefix) {
				t.Errorf("export file for %s is outside the declared cache: %s", result.ID, result.ExportFile)
				continue
			}
			exportFile := filepath.Join(cacheDir, strings.TrimPrefix(result.ExportFile, cachePrefix))
			if _, err := os.Stat(exportFile); err != nil {
				t.Errorf("export file referenced by %s is missing: %v", result.ID, err)
			}
			referenced[exportFile] = true
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
	if err := filepath.Walk(cacheDir, func(file string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() && info.Name() != "README" && !referenced[file] {
			t.Errorf("cache contains an unreferenced file: %s", file)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
}
