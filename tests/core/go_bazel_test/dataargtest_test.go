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

package main

import (
	"bytes"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/bazelbuild/rules_go/go/runfiles"
	"github.com/bazelbuild/rules_go/go/tools/bazel"
	"github.com/bazelbuild/rules_go/go/tools/bazel_testing"
)

var (
	binaryPath           = flag.String("binaryPath", "", "")
	expectedBazel        = flag.String("expectedBazel", "", "")
	expectedBazelVersion = flag.String("expectedBazelVersion", "", "")
	originalTmpDir       = os.Getenv("TEST_TMPDIR")
	setUpRan             = false
)

func TestMain(m *testing.M) {
	bazel_testing.TestMain(m, bazel_testing.Args{
		Main: `
-- root.txt --
Hello world!
-- nested/file.txt --
Hello world!`,
		SetUp: func() error {
			setUpRan = true
			return nil
		},
	})
}

// Tests that go_bazel_test keeps includes data files correctly and doesn't mess
// up on `args` that include `$(location ...)` calls.
func TestGoldenPath(t *testing.T) {
	bp, err := bazel.Runfile(*binaryPath)
	if err != nil {
		t.Fatalf("unable to get the runfile path %#v: %s", *binaryPath, err)
	}

	tests := map[string]string{
		"Go binary file":          bp,
		"Text file in root":       "root.txt",
		"Text file in nested dir": "nested/file.txt",
	}

	for name, f := range tests {
		_, err = os.Stat(f)
		if err != nil {
			t.Fatalf("unable to stat %s file (%q): %s", name, f, err)
		}
	}

	if setUpRan == false {
		t.Fatal("setUp should have been executed but was not")
	}
}

// The selected Bazel must remain usable after changing to the generated
// workspace and without a Bazel installation on PATH.
func TestDeclaredBazel(t *testing.T) {
	want, err := runfiles.Rlocation(*expectedBazel)
	if err != nil {
		t.Fatal(err)
	}
	want, err = filepath.Abs(want)
	if err != nil {
		t.Fatal(err)
	}

	t.Setenv("PATH", t.TempDir())
	cmd := bazel_testing.BazelCmd("info", "output_base")
	if cmd.Path != want {
		t.Fatalf("BazelCmd executable = %q, want declared runfile %q", cmd.Path, want)
	}
	versionFile, err := runfiles.Rlocation(*expectedBazelVersion)
	if err != nil {
		t.Fatal(err)
	}
	wantVersion, err := os.ReadFile(versionFile)
	if err != nil {
		t.Fatal(err)
	}
	// --version cannot be combined with the startup options in BazelCmd.
	// Invoke the selected executable directly with the same isolated environment.
	versionCmd := exec.Command(cmd.Path, "--version")
	versionCmd.Env = cmd.Env
	versionCmd.Stderr = os.Stderr
	gotVersion, err := versionCmd.Output()
	if err != nil {
		t.Fatalf("could not query the declared Bazel version without PATH: %v", err)
	}
	if !bytes.Equal(gotVersion, wantVersion) {
		t.Fatalf("declared Bazel --version = %q, want enclosing Bazel version %q", gotVersion, wantVersion)
	}
	wantMajor := 0
	var numericMajor int
	if _, err := fmt.Sscanf(string(wantVersion), "bazel %d.", &numericMajor); err == nil {
		wantMajor = numericMajor
	}
	if version := bazel_testing.BazelMajorVersion(); version != wantMajor {
		t.Fatalf("BazelMajorVersion() = %d, want %d for %q", version, wantMajor, wantVersion)
	}
	cmd.Stderr = os.Stderr
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("declared Bazel failed without PATH: %v", err)
	}
	outputBase := strings.TrimSpace(string(out))
	assertInTestTmpDir(t, "output_base", outputBase)

	// One test may run Bazel in multiple fixture workspaces. They must use
	// different output bases even though the enclosing test owns both.
	secondWorkspace := t.TempDir()
	if err := os.WriteFile(filepath.Join(secondWorkspace, "MODULE.bazel"), []byte("module(name = \"second_workspace\")\n"), 0666); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		shutdown := bazel_testing.BazelCmd("shutdown")
		shutdown.Dir = secondWorkspace
		if out, err := shutdown.CombinedOutput(); err != nil {
			t.Errorf("shutting down second fixture workspace: %v\n%s", err, out)
		}
	})
	second := bazel_testing.BazelCmd("info", "output_base")
	second.Dir = secondWorkspace
	second.Stderr = os.Stderr
	out, err = second.Output()
	if err != nil {
		t.Fatalf("Bazel failed in the second fixture workspace: %v", err)
	}
	secondOutputBase := strings.TrimSpace(string(out))
	assertInTestTmpDir(t, "second output_base", secondOutputBase)
	if secondOutputBase == outputBase {
		t.Fatalf("different fixture workspaces share output base %q", outputBase)
	}
}

func TestIsolatedWorkspace(t *testing.T) {
	workspace, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	assertInTestTmpDir(t, "workspace", workspace)

	env := make(map[string]string)
	for _, entry := range bazel_testing.BazelCmd("info").Env {
		key, value, _ := strings.Cut(entry, "=")
		env[key] = value
	}
	for _, name := range []string{
		"HOME", "USERPROFILE", "XDG_CACHE_HOME", "TMPDIR", "TMP", "TEMP",
		"GOCACHE", "GOPATH", "GOMODCACHE", "BAZELISK_HOME", "TEST_TMPDIR",
	} {
		assertInTestTmpDir(t, name, env[name])
		info, err := os.Stat(env[name])
		if err != nil {
			t.Errorf("%s is not writable test storage: %v", name, err)
			continue
		}
		if !info.IsDir() {
			t.Errorf("%s = %q is not a directory", name, env[name])
			continue
		}
		f, err := os.CreateTemp(env[name], "write-test-")
		if err != nil {
			t.Errorf("%s is not writable test storage: %v", name, err)
			continue
		}
		f.Close()
		os.Remove(f.Name())
	}
}

func assertInTestTmpDir(t *testing.T, name, path string) {
	t.Helper()
	if originalTmpDir == "" {
		t.Fatal("TEST_TMPDIR was not set for the enclosing test")
	}
	if !filepath.IsAbs(path) {
		t.Fatalf("%s = %q is not absolute", name, path)
	}
	root, err := filepath.EvalSymlinks(originalTmpDir)
	if err != nil {
		t.Fatal(err)
	}
	resolved, err := filepath.EvalSymlinks(path)
	if err != nil {
		t.Fatal(err)
	}
	rel, err := filepath.Rel(root, resolved)
	if err != nil {
		t.Fatal(err)
	}
	if rel == "." || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		t.Errorf("%s = %q is not private storage under TEST_TMPDIR %q", name, path, originalTmpDir)
	}
}
