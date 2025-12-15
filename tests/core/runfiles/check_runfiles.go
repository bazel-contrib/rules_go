// Copyright 2019 The Bazel Authors. All rights reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//    http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package check

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"

	"github.com/bazelbuild/rules_go/go/tools/bazel"
)

type TestFile struct {
	Workspace, ShortPath, Path string
	Binary                     bool
}

var DefaultTestFiles = []TestFile{
	{Workspace: "io_bazel_rules_go", Path: "tests/core/runfiles/local_file.txt"},
	{Workspace: "io_bazel_rules_go", Path: "tests/core/runfiles/local_group.txt"},
	{Workspace: "io_bazel_rules_go", Path: "tests/core/runfiles/local_bin", Binary: true},
	{Workspace: "runfiles_remote_test", Path: "remote_pkg/remote_file.txt"},
	{Workspace: "runfiles_remote_test", Path: "remote_pkg/remote_group.txt"},
	{Workspace: "runfiles_remote_test", Path: "remote_pkg/remote_bin", Binary: true},
}

func CheckRunfiles(files []TestFile) error {
	// Check that the runfiles directory matches the current workspace.
	// There is no runfiles directory on Windows.
	if runtime.GOOS != "windows" {
		dir, err := bazel.RunfilesPath()
		if err != nil {
			return err
		}
		root, base := filepath.Dir(dir), filepath.Base(dir)
		if !strings.HasSuffix(root, ".runfiles") {
			return fmt.Errorf("RunfilesPath: %q is not a .runfiles directory", dir)
		}
		workspace := os.Getenv("TEST_WORKSPACE")
		if workspace != "" && workspace != base {
			return fmt.Errorf("RunfilesPath: %q does not match test workspace %s", dir, workspace)
		}
		if srcDir := os.Getenv("TEST_SRCDIR"); srcDir != "" && filepath.Join(srcDir, workspace) != dir {
			return fmt.Errorf("RunfilesPath: %q does not match TEST_SRCDIR %q", dir, srcDir)
		}
	}

	// Check that files can be found with Runfile or FindBinary.
	// Make sure the paths returned are absolute paths to actual files.
	seen := make(map[string]string)
	for _, f := range files {
		var got string
		var err error
		if !f.Binary {
			if got, err = bazel.Runfile(f.Path); err != nil {
				return err
			}
			if !filepath.IsAbs(got) {
				return fmt.Errorf("Runfile %s: got a relative path %q; want absolute", f.Path, got)
			}
			seen[f.Path] = got
		} else {
			var pkg, name string
			if i := strings.LastIndex(f.Path, "/"); i < 0 {
				name = f.Path
			} else {
				pkg = f.Path[:i]
				name = f.Path[i+1:]
			}
			var ok bool
			if got, ok = bazel.FindBinary(pkg, name); !ok {
				return fmt.Errorf("FindBinary %s %s: could not find binary", pkg, name)
			}
			if !filepath.IsAbs(got) {
				return fmt.Errorf("FindBinary %s %s: got a relative path %q; want absolute", pkg, name, got)
			}
		}

		if _, err := os.Stat(got); err != nil {
			return fmt.Errorf("%s: could not stat: %v", f.Path, err)
		}
	}

	// Check that the files can be listed.
	entries, err := bazel.ListRunfiles()
	if err != nil {
		return err
	}
	for _, e := range entries {
		if want, ok := seen[e.ShortPath]; ok && want != e.Path {
			return err
		}
		delete(seen, e.ShortPath)
	}

	if len(seen) > 0 {
		unseen := make([]string, 0, len(seen))
		for short := range seen {
			unseen = append(unseen, short)
		}
		sort.Strings(unseen)
		return fmt.Errorf("ListRunfiles did not include files:\n\t%s", strings.Join(unseen, "\n\t"))
	}

	return nil
}
