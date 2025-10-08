package output_groups

import (
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"testing"

	"github.com/bazelbuild/rules_go/go/tools/bazel"
)

func TestCompilationOutputs(t *testing.T) {
	runfiles, err := bazel.ListRunfiles()
	if err != nil {
		t.Fatal(err)
	}

	exe := ""
	if runtime.GOOS == "windows" {
		exe = ".exe"
	}
	expectedFiles := map[string]bool{
		"compilation_outputs_test" + exe:                   true, // test binary; not relevant
		"compilation_outputs_test" + exe + ".repo_mapping": true, // test binary repo mapping; not relevant

		"lib.a":                    false, // :lib archive
		"lib_test.internal.a":      false, // :lib_test internal archive
		"lib_test_test.external.a": false, // :lib_test external archive
		"bin.a":                    false, // :bin archive
	}
	for _, rf := range runfiles {
		info, err := os.Stat(rf.Path)
		if err != nil {
			t.Error(err)
			continue
		}
		if info.IsDir() {
			continue
		}

		base := filepath.Base(rf.Path)
		if seen, ok := expectedFiles[base]; !ok {
			t.Errorf("unexpected runfile: %s %s", rf.Path, base)
		} else if !seen {
			expectedFiles[base] = true
		}
	}

	missingFiles := make([]string, 0, len(expectedFiles))
	for path, seen := range expectedFiles {
		if !seen {
			missingFiles = append(missingFiles, path)
		}
	}
	sort.Strings(missingFiles)
	if len(missingFiles) > 0 {
		t.Errorf("did not find expected files: %s", strings.Join(missingFiles, " "))
	}
}
