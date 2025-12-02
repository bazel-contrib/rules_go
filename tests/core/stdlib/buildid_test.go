//go:build go1.10
// +build go1.10

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
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/bazelbuild/rules_go/go/runfiles"
)

func TestEmptyBuildID(t *testing.T) {
	// Locate the buildid tool and several archive files to check.
	//   fmt.a - pure go
	//   crypto/aes.a - contains assembly
	//   net.a - contains cgo
	// The path may vary depending on platform and architecture, so just
	// do a search.
	var goPath string
	pkgPaths := map[string]string{
		"fmt.a": "",
		"aes.a": "",
		"net.a": "",
	}
	stdlibPkgDir, err := runfiles.Rlocation("_main/stdlib_/pkg")
	if err != nil {
		t.Fatal(err)
	}
	n := len(pkgPaths)
	done := errors.New("done")
	var visit filepath.WalkFunc
	visit = func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if filepath.Base(path) == "go" && (info.Mode()&0111) != 0 {
			goPath = path
		}
		for pkg := range pkgPaths {
			if filepath.Base(path) == pkg {
				pkgPaths[pkg] = path
				n--
			}
		}
		if goPath != "" && n == 0 {
			return done
		}
		return nil
	}
	if err = filepath.Walk(stdlibPkgDir, visit); err != nil && err != done {
		t.Fatal(err)
	}
	if goPath == "" {
		t.Fatal("go not found")
	}

	for pkg, path := range pkgPaths {
		if path == "" {
			t.Errorf("could not locate %s", pkg)
			continue
		}
		// It's an error if this produces any output.
		cmd := exec.Command(goPath, "tool", "buildid", path)
		out, err := cmd.Output()
		if err != nil {
			t.Error(err)
		}
		if len(bytes.TrimSpace(out)) > 0 {
			t.Errorf("%s: unexpected buildid: %s", path, out)
		}
	}
}
