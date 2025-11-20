// Copyright 2020 The Bazel Authors. All rights reserved.
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

package testmain_pwd_test

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

var (
	exeSuffix      string // set in TestMain only on Windows
	testExecutable string // set in TestMain if we're running tests
)

func TestMain(m *testing.M) {
	if runtime.GOOS == "windows" {
		exeSuffix = ".exe"
	}

	if filepath.Base(os.Args[0]) == "pwd"+exeSuffix {
		cwd, err := os.Getwd()
		if err != nil {
			log.Fatalf("failed to get current working directory: %v", err)
		}

		fmt.Println(cwd)
		os.Exit(0)
	}

	var err error
	testExecutable, err = os.Executable()
	if err != nil {
		log.Fatalf("failed to get executable path: %v", err)
	}

	os.Exit(m.Run())
}

// Creates a $tmp/pwd symlink to the test executable and run it.
func TestSymlinkToBinary(t *testing.T) {
	workDir := t.TempDir()

	pwdExe := filepath.Join(workDir, "pwd"+exeSuffix)
	if err := os.Symlink(testExecutable, pwdExe); err != nil {
		t.Fatalf("failed to create symlink: %v", err)
	}

	cmd := exec.Command(pwdExe)
	cmd.Dir = workDir
	bs, err := cmd.Output()
	if err != nil {
		t.Fatalf("failed to run symlinked executable: %v", err)
	}

	gotPath := strings.TrimSpace(string(bs))
	if !sameFile(t, gotPath, workDir) {
		t.Fatalf("subprocess inside the incorrect directory:\n"+
			"want: %s\n got: %s", workDir, gotPath)
	}
}

func TestChangeOSArgs0(t *testing.T) {
	workDir := t.TempDir()

	cmd := exec.Command(testExecutable)
	cmd.Args[0] = "pwd" + exeSuffix
	cmd.Dir = workDir
	bs, err := cmd.Output()
	if err != nil {
		t.Fatalf("failed to run symlinked executable: %v", err)
	}

	gotPath := strings.TrimSpace(string(bs))
	if !sameFile(t, gotPath, workDir) {
		t.Fatalf("subprocess inside the incorrect directory:\n"+
			"want: %s\n got: %s", workDir, gotPath)
	}
}

// Reports whether two paths point to the same file or directory.
func sameFile(t testing.TB, a, b string) bool {
	aStat, err := os.Stat(a)
	if err != nil {
		t.Fatalf("failed to stat %q: %v", a, err)
	}
	bStat, err := os.Stat(b)
	if err != nil {
		t.Fatalf("failed to stat %q: %v", b, err)
	}
	return os.SameFile(aStat, bStat)
}
