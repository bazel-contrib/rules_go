// Copyright 2021 Google LLC
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
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"

	"github.com/bazelbuild/rules_go/go/runfiles"
	"github.com/bazelbuild/rules_go/go/tests/runfiles/testfs"
)

var isBzlmodEnabled bool

func init() {
	repoMapping, err := runfiles.Rlocation("_repo_mapping")
	if err != nil {
		return
	}
	content, err := os.ReadFile(repoMapping)
	if err != nil {
		return
	}
	isBzlmodEnabled = strings.Contains(string(content), ",io_bazel_rules_go,_main\n")
}

func mainRepoRunfiles(useCanonicalName bool) []string {
	var mainRepo string
	if useCanonicalName && isBzlmodEnabled {
		mainRepo = "_main"
	} else {
		mainRepo = "io_bazel_rules_go"
	}
	exeSuffix := ""
	if runtime.GOOS == "windows" {
		exeSuffix = ".exe"
	}
	return []string{
		mainRepo + "/tests/runfiles/runfiles_test_/runfiles_test" + exeSuffix,
		mainRepo + "/tests/runfiles/test.txt",
		mainRepo + "/tests/runfiles/test_dir",
		mainRepo + "/tests/runfiles/testprog/testprog_/testprog" + exeSuffix,
	}
}

var commonRunfiles = []string{
	"_repo_mapping",
	"bazel_tools/tools/bash/runfiles/runfiles.bash",
	"link_test.txt",
	"link_test_dir/file.txt",
	"link_test_dir/subdir/other_file.txt",
}

func TestFS_native(t *testing.T) {
	r, err := runfiles.New()
	if err != nil {
		t.Fatal(err)
	}
	testFS(t, r)
}

func TestFS_directory(t *testing.T) {
	// Turn our own runfiles (whatever the form) into a valid runfiles directory.
	tempdir := t.TempDir()
	directory := filepath.Join(tempdir, "directory")
	err := os.Mkdir(directory, 0o755)
	if err != nil {
		t.Fatal(err)
	}
	for _, source := range append(mainRepoRunfiles(true), commonRunfiles...) {
		target, err := runfiles.Rlocation(source)
		if err != nil {
			t.Fatal(err)
		}
		err = os.MkdirAll(filepath.Dir(filepath.Join(directory, source)), 0o755)
		if err != nil {
			t.Fatal(err)
		}
		err = os.Symlink(target, filepath.Join(directory, source))
		if err != nil {
			t.Fatal(err)
		}
	}

	fsys, err := runfiles.New(runfiles.Directory(directory))
	if err != nil {
		t.Fatal(err)
	}
	testFS(t, fsys)
}

func TestFS_manifest(t *testing.T) {
	// Turn our own runfiles (whatever the form) into a valid manifest file.
	tempdir := t.TempDir()
	manifest := filepath.Join(tempdir, "manifest")
	manifestFile, err := os.Create(manifest)
	if err != nil {
		t.Fatal(err)
	}
	for _, source := range append(mainRepoRunfiles(true), commonRunfiles...) {
		target, err := runfiles.Rlocation(source)
		if err != nil {
			t.Fatal(err)
		}
		_, err = manifestFile.WriteString(source + " " + target + "\n")
		if err != nil {
			t.Fatal(err)
		}
	}
	if err = manifestFile.Close(); err != nil {
		t.Fatal(err)
	}

	fsys, err := runfiles.New(runfiles.ManifestFile(manifest))
	if err != nil {
		t.Fatal(err)
	}
	testFS(t, fsys)
}

func testFS(t *testing.T, r *runfiles.Runfiles) {
	// Ensure that the Runfiles object implements FS interfaces.
	var _ fs.FS = r

	runfilesToTest := commonRunfiles
	runfilesToTest = append(runfilesToTest, mainRepoRunfiles(true)...)
	if isBzlmodEnabled {
		runfilesToTest = append(runfilesToTest, mainRepoRunfiles(false)...)
	}
	if err := testfs.TestFS(r, runfilesToTest...); err != nil {
		t.Error(err)
	}

	if isBzlmodEnabled {
		testFile(t, r, "_main/tests/runfiles/test.txt", "hi!\n")
		testFile(t, r, "_main/tests/runfiles/test_dir/file.txt", "file\n")
		testFile(t, r, "_main/tests/runfiles/test_dir/subdir/other_file.txt", "other_file\n")
	}
	testFile(t, r, "io_bazel_rules_go/tests/runfiles/test.txt", "hi!\n")
	testFile(t, r, "io_bazel_rules_go/tests/runfiles/test_dir/file.txt", "file\n")
	testFile(t, r, "io_bazel_rules_go/tests/runfiles/test_dir/subdir/other_file.txt", "other_file\n")
	testFile(t, r, "link_test.txt", "hi!\n")
	testFile(t, r, "link_test_dir/file.txt", "file\n")
	testFile(t, r, "link_test_dir/subdir/other_file.txt", "other_file\n")

	testGlob(t, r)
	testWalkDir(t, r)
}

func testFile(t *testing.T, r *runfiles.Runfiles, name, content string) {
	f, err := r.Open(name)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		t.Fatal(err)
	}
	if info.Name() != path.Base(name) {
		t.Errorf("Name: got %q, want %q", info.Name(), path.Base(name))
	}
	if info.IsDir() {
		t.Errorf("IsDir: got %v, want %v", info.IsDir(), false)
	}
	if !info.Mode().IsRegular() {
		t.Errorf("IsRegular: got %v, want %v", info.Mode().IsRegular(), true)
	}
	if info.Size() != int64(len(content)) {
		t.Errorf("Size: got %d, want %d", info.Size(), len(content))
	}
	got, err := fs.ReadFile(r, name)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != content {
		t.Errorf("got %q, want %q", got, content)
	}
}

func testGlob(t *testing.T, r *runfiles.Runfiles) {
	matches, err := fs.Glob(r, "*/subdir/*.txt")
	if err != nil {
		t.Fatal(err)
	}

	expected := []string{
		"link_test_dir/subdir/other_file.txt",
	}
	if !slices.Equal(matches, expected) {
		t.Errorf("got %v, want %v", matches, expected)
	}
}

func testWalkDir(t *testing.T, r *runfiles.Runfiles) {
	var found []string
	err := fs.WalkDir(r, "io_bazel_rules_go/tests/runfiles", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() {
			found = append(found, path)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}

	exeSuffix := ""
	if runtime.GOOS == "windows" {
		exeSuffix = ".exe"
	}
	expected := []string{
		"io_bazel_rules_go/tests/runfiles/runfiles_test_/runfiles_test" + exeSuffix,
		"io_bazel_rules_go/tests/runfiles/test.txt",
		"io_bazel_rules_go/tests/runfiles/test_dir/file.txt",
		"io_bazel_rules_go/tests/runfiles/test_dir/subdir/other_file.txt",
		"io_bazel_rules_go/tests/runfiles/testprog/testprog_/testprog" + exeSuffix,
	}
	if !slices.Equal(found, expected) {
		t.Errorf("got %v, want %v", found, expected)
	}
}

func TestFS_empty(t *testing.T) {
	dir := t.TempDir()
	manifest := filepath.Join(dir, "manifest")
	if err := os.WriteFile(manifest, []byte("__init__.py \n"), 0o600); err != nil {
		t.Fatal(err)
	}
	fsys, err := runfiles.New(runfiles.ManifestFile(manifest), runfiles.ProgramName("/invalid"), runfiles.Directory("/invalid"))
	if err != nil {
		t.Fatal(err)
	}
	t.Run("Open", func(t *testing.T) {
		fd, err := fsys.Open("__init__.py")
		if err != nil {
			t.Fatal(err)
		}
		defer fd.Close()
		got, err := io.ReadAll(fd)
		if err != nil {
			t.Error(err)
		}
		if len(got) != 0 {
			t.Errorf("got nonempty contents: %q", got)
		}
	})
	t.Run("Stat", func(t *testing.T) {
		got, err := fs.Stat(fsys, "__init__.py")
		if err != nil {
			t.Fatal(err)
		}
		if got.Name() != "__init__.py" {
			t.Errorf("Name: got %q, want %q", got.Name(), "__init__.py")
		}
		if got.Size() != 0 {
			t.Errorf("Size: got %d, want %d", got.Size(), 0)
		}
		if !got.Mode().IsRegular() {
			t.Errorf("IsRegular: got %v, want %v", got.Mode().IsRegular(), true)
		}
		if got.IsDir() {
			t.Errorf("IsDir: got %v, want %v", got.IsDir(), false)
		}
	})
	t.Run("ReadFile", func(t *testing.T) {
		got, err := fs.ReadFile(fsys, "__init__.py")
		if err != nil {
			t.Error(err)
		}
		if len(got) != 0 {
			t.Errorf("got nonempty contents: %q", got)
		}
	})
}
