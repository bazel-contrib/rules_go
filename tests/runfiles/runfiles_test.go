// Copyright 2020, 2021, 2022 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     https://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package runfiles_test

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"

	"github.com/bazelbuild/rules_go/go/runfiles"
)

func TestPath_FileLookup(t *testing.T) {
	path, err := runfiles.Rlocation("io_bazel_rules_go/tests/runfiles/test.txt")
	if err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	got := strings.TrimSpace(string(b))
	want := "hi!"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestPath_SubprocessRunfilesLookup(t *testing.T) {
	r, err := runfiles.New()
	if err != nil {
		panic(err)
	}
	// The binary “testprog” is itself built with Bazel, and needs
	// runfiles.
	testprogRpath := "io_bazel_rules_go/tests/runfiles/testprog/testprog_/testprog"
	if runtime.GOOS == "windows" {
		testprogRpath += ".exe"
	}
	prog, err := r.Rlocation(testprogRpath)
	if err != nil {
		panic(err)
	}
	cmd := exec.Command(prog)
	// We add r.Env() after os.Environ() so that runfile environment
	// variables override anything set in the process environment.
	cmd.Env = append(os.Environ(), r.Env()...)
	out, err := cmd.Output()
	if err != nil {
		t.Fatal(err)
	}
	got := strings.TrimSpace(string(out))
	want := "hi!"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestPath_errors(t *testing.T) {
	r, err := runfiles.New()
	if err != nil {
		t.Fatal(err)
	}
	for _, s := range []string{"", "/..", "../", "a/../b", "a//b", "a/./b", `\a`} {
		t.Run(s, func(t *testing.T) {
			if got, err := r.Rlocation(s); err == nil {
				t.Errorf("got %q, want error", got)
			}
		})
	}
	for _, s := range []string{"foo/..bar", "foo/.bar"} {
		t.Run(s, func(t *testing.T) {
			if _, err := r.Rlocation(s); err != nil && !os.IsNotExist(err.(runfiles.Error).Err) {
				t.Errorf("got %q, want none or 'file not found' error", err)
			}
		})
	}
}

func TestRunfiles_zero(t *testing.T) {
	var r runfiles.Runfiles
	if got, err := r.Rlocation("a"); err == nil {
		t.Errorf("Rlocation: got %q, want error", got)
	}
	if got := r.Env(); got != nil {
		t.Errorf("Env: got %v, want nil", got)
	}
}

func TestRunfiles_empty(t *testing.T) {
	dir := t.TempDir()
	manifest := filepath.Join(dir, "manifest")
	if err := os.WriteFile(manifest, []byte("__init__.py \n"), 0o600); err != nil {
		t.Fatal(err)
	}
	r, err := runfiles.New(runfiles.ManifestFile(manifest))
	if err != nil {
		t.Fatal(err)
	}
	_, got := r.Rlocation("__init__.py")
	want := runfiles.ErrEmpty
	if !errors.Is(got, want) {
		t.Errorf("Rlocation for empty file: got error %q, want something that wraps %q", got, want)
	}
}

func TestRunfiles_manifestWithDir(t *testing.T) {
	dir := t.TempDir()
	manifest := filepath.Join(dir, "manifest")
	if err := os.WriteFile(manifest, []byte(`foo/dir path/to/foo/dir
 dir\swith\sspac\be\ns F:\bj k\bdir with spa\nces
`), 0o600); err != nil {
		t.Fatal(err)
	}
	r, err := runfiles.New(runfiles.ManifestFile(manifest))
	if err != nil {
		t.Fatal(err)
	}

	for rlocation, want := range map[string]string{
		"foo/dir":                    filepath.FromSlash("path/to/foo/dir"),
		"foo/dir/file":               filepath.FromSlash("path/to/foo/dir/file"),
		"foo/dir/deeply/nested/file": filepath.FromSlash("path/to/foo/dir/deeply/nested/file"),
		`dir with spac\e
s`:            filepath.FromSlash(`F:\j k\dir with spa
ces`),
		`dir with spac\e
s/file`:            filepath.FromSlash(`F:\j k\dir with spa
ces/file`),
	} {
		t.Run(rlocation, func(t *testing.T) {
			got, err := r.Rlocation(rlocation)
			if err != nil {
				t.Fatalf("Rlocation failed: got unexpected error %q", err)
			}
			if got != want {
				t.Errorf("Rlocation failed: got %q, want %q", got, want)
			}
		})
	}
}

func TestRunfiles_dirEnv(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Windows doesn't have a runfiles directory by default")
	}

	dir := t.TempDir()
	r, err := runfiles.New(runfiles.Directory(dir))
	if err != nil {
		t.Fatal(err)
	}

	want := []string{"RUNFILES_DIR=" + dir, "JAVA_RUNFILES=" + dir}
	if !reflect.DeepEqual(r.Env(), want) {
		t.Errorf("Env: got %v, want %v", r.Env(), want)
	}
}

func TestRunfiles_manifestEnv(t *testing.T) {
	tmp := t.TempDir()
	dir := filepath.Join(tmp, "foo.runfiles")
	err := os.Mkdir(dir, 0o755)
	if err != nil {
		t.Fatal(err)
	}
	manifest := filepath.Join(tmp, "foo.runfiles_manifest")
	if err = os.WriteFile(manifest, []byte("foo/dir path/to/foo/dir\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	r, err := runfiles.New(runfiles.ManifestFile(manifest))
	if err != nil {
		t.Fatal(err)
	}

	want := []string{"RUNFILES_MANIFEST_FILE=" + manifest, "RUNFILES_DIR=" + dir, "JAVA_RUNFILES=" + dir}
	if !reflect.DeepEqual(r.Env(), want) {
		t.Errorf("Env: got %v, want %v", r.Env(), want)
	}
}
