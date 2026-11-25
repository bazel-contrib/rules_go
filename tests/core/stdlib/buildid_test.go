/* Copyright 2018 The Bazel Authors. All rights reserved.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

   http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package buildid_test

import (
	"bytes"
	"errors"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/bazelbuild/rules_go/go/runfiles"
)

func TestEmptyBuildID(t *testing.T) {
	goPath, err := runfiles.Rlocation("go_default_sdk/bin/go")
	if err != nil {
		t.Fatal(err)
	}

	// Locate the buildid tool and several archive files to check.
	//   fmt.a - pure go
	//   crypto/aes.a - contains assembly
	//   net.a - contains cgo
	// The path may vary depending on platform and architecture, so just
	// do a search.
	pkgPaths := map[string]string{
		"fmt.a": "",
		"aes.a": "",
		"net.a": "",
	}
	toFind := make(map[string]struct{})
	for k := range pkgPaths {
		toFind[k] = struct{}{}
	}

	stdlibPkgDir, err := runfiles.Rlocation("_main/stdlib_/pkg")
	if err != nil {
		t.Fatal(err)
	}

	done := errors.New("done")
	var visit fs.WalkDirFunc
	visit = func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		for pkg := range toFind {
			if d.Name() == pkg {
				pkgPaths[d.Name()] = path
				delete(toFind, pkg)
			}
		}
		if len(toFind) == 0 {
			return done
		}
		return nil
	}
	if err = filepath.WalkDir(stdlibPkgDir, visit); err != nil && err != done {
		t.Fatal(err)
	}

	for pkg, path := range pkgPaths {
		if path == "" {
			t.Errorf("could not locate %s", pkg)
			continue
		}
		// It's an error if this produces any output.
		cmd := exec.Command(goPath, "tool", "buildid", path)
		// Go 1.25 and later initialize the build cache even for commands that
		// don't use it, which fails in the test sandbox as neither GOCACHE nor
		// HOME is set.
		cmd.Env = append(os.Environ(), "GOCACHE="+t.TempDir())
		out, err := cmd.Output()
		if err != nil {
			t.Error(err)
		}
		if len(bytes.TrimSpace(out)) > 0 {
			t.Errorf("%s: unexpected buildid: %s", path, out)
		}
	}
}
