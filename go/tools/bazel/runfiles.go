// Copyright 2018 The Bazel Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package bazel

import (
	"bytes"
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"
)

const (
	RUNFILES_MANIFEST_FILE = "RUNFILES_MANIFEST_FILE"
	RUNFILES_DIR           = "RUNFILES_DIR"
)

// Runfile returns an absolute path to the file named by "path", which
// should be a relative path from the workspace root to the file within
// the bazel workspace.
//
// Runfile may be called from tests invoked with 'bazel test' and
// binaries invoked with 'bazel run'. On Windows,
// only tests invoked with 'bazel test' are supported.
//
// Deprecated: Use [github.com/bazelbuild/rules_go/go/runfiles] instead for
// cross-platform support matching the behavior of the Bazel-provided runfiles
// libraries.
func Runfile(path string) (string, error) {
	// Search in working directory
	if _, err := os.Stat(path); err == nil {
		return filepath.Abs(path)
	}

	if err := ensureRunfiles(); err != nil {
		return "", err
	}

	// Search manifest if we have one.
	if entry, ok := runfiles.index.GetIgnoringWorkspace(path); ok {
		return entry.Path, nil
	}

	if strings.HasPrefix(path, "../") || strings.HasPrefix(path, "external/") {
		pathParts := strings.Split(path, "/")
		if len(pathParts) >= 3 {
			workspace := pathParts[1]
			pathInsideWorkspace := strings.Join(pathParts[2:], "/")
			if path := runfiles.index.Get(workspace, pathInsideWorkspace); path != "" {
				return path, nil
			}
		}
	}

	// Search the main workspace.
	if runfiles.workspace != "" {
		mainPath := filepath.Join(runfiles.dir, runfiles.workspace, path)
		if _, err := os.Stat(mainPath); err == nil {
			return mainPath, nil
		}
	}

	// Search other workspaces.
	for _, w := range runfiles.workspaces {
		workPath := filepath.Join(runfiles.dir, w, path)
		if _, err := os.Stat(workPath); err == nil {
			return workPath, nil
		}
	}

	return "", fmt.Errorf("Runfile %s: could not locate file", path)
}

// FindBinary returns an absolute path to the binary built from a go_binary
// rule in the given package with the given name. FindBinary is similar to
// [Runfile], but it accounts for varying configurations and file extensions,
// which may cause the binary to have different paths on different platforms.
//
// FindBinary may be called from tests invoked with 'bazel test' and
// binaries invoked with 'bazel run'. On Windows,
// only tests invoked with 'bazel test' are supported.
//
// Deprecated: Use [github.com/bazelbuild/rules_go/go/runfiles.Rlocation]
// instead.  The path argument can be obtained by passing
// $(rlocationpath //pkg:name) to the go_* target via args, env or x_defs
// (the latter is also available on go_library).  This avoids hardcoding the
// package and name of the binary in the source code.
func FindBinary(pkg, name string) (string, bool) {
	if err := ensureRunfiles(); err != nil {
		return "", false
	}

	// If we've gathered a list of runfiles, either by calling ListRunfiles or
	// parsing the manifest on Windows, just use that instead of searching
	// directories. Return the first match. The manifest on Windows may contain
	// multiple entries for the same file.
	if runfiles.list != nil {
		if runtime.GOOS == "windows" {
			name += ".exe"
		}
		for _, entry := range runfiles.list {
			if path.Base(entry.ShortPath) != name {
				continue
			}
			pkgDir := path.Dir(path.Dir(entry.ShortPath))
			if pkgDir == "." {
				pkgDir = ""
			}
			if pkgDir != pkg {
				continue
			}
			return entry.Path, true
		}
		return "", false
	}

	dir, err := Runfile(pkg)
	if err != nil {
		return "", false
	}
	var found string
	stopErr := errors.New("stop")
	err = filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		base := filepath.Base(path)
		stem := strings.TrimSuffix(base, ".exe")
		if stem != name {
			return nil
		}
		if runtime.GOOS != "windows" {
			if st, err := os.Stat(path); err != nil {
				return err
			} else if st.Mode()&0111 == 0 {
				return nil
			}
		}
		if stem == name {
			found = path
			return stopErr
		}
		return nil
	})
	if err == stopErr {
		return found, true
	} else {
		return "", false
	}
}

// A RunfileEntry describes a runfile.
//
// Deprecated: See comment on [ListRunfiles].
type RunfileEntry struct {
	// Workspace is the bazel workspace the file came from. For example,
	// this would be "io_bazel_rules_go" for a file in rules_go.
	Workspace string

	// ShortPath is a relative, slash-separated path from the workspace root
	// to the file. For non-binary files, this may be passed to Runfile
	// to locate a file.
	ShortPath string

	// Path is an absolute path to the file.
	Path string
}

// ListRunfiles returns a list of available runfiles.
//
// Deprecated: Use [fs.Glob] or [fs.WalkDir] on the [fs.FS] implementation
// provided by [github.com/bazelbuild/rules_go/go/runfiles.New] instead.
func ListRunfiles() ([]RunfileEntry, error) {
	if err := ensureRunfiles(); err != nil {
		return nil, err
	}

	if runfiles.list == nil && runfiles.dir != "" {
		runfiles.listOnce.Do(func() {
			var list []RunfileEntry
			haveWorkspaces := strings.HasSuffix(runfiles.dir, ".runfiles") && runfiles.workspace != ""

			err := filepath.Walk(runfiles.dir, func(path string, info os.FileInfo, err error) error {
				if err != nil {
					return err
				}
				rel, _ := filepath.Rel(runfiles.dir, path)
				rel = filepath.ToSlash(rel)
				if rel == "." {
					return nil
				}

				var workspace, shortPath string
				if haveWorkspaces {
					if i := strings.IndexByte(rel, '/'); i < 0 {
						return nil
					} else {
						workspace, shortPath = rel[:i], rel[i+1:]
					}
				} else {
					workspace, shortPath = "", rel
				}

				list = append(list, RunfileEntry{Workspace: workspace, ShortPath: shortPath, Path: path})
				return nil
			})
			if err != nil {
				runfiles.err = err
				return
			}
			runfiles.list = list
		})
	}
	return runfiles.list, runfiles.err
}

// TestWorkspace returns the name of the Bazel workspace for this test.
// TestWorkspace returns an error if the TEST_WORKSPACE environment variable
// was not set or [SetDefaultTestWorkspace] was not called.
//
// Deprecated: With Bzlmod enabled, the workspace name is always "_main".  Use
// [github.com/bazelbuild/rules_go/go/runfiles] instead to access runfiles.
func TestWorkspace() (string, error) {
	if err := ensureRunfiles(); err != nil {
		return "", err
	}
	if runfiles.workspace != "" {
		return runfiles.workspace, nil
	}
	return "", errors.New("TEST_WORKSPACE not set and SetDefaultTestWorkspace not called")
}

// SetDefaultTestWorkspace allows you to set a fake value for the
// environment variable TEST_WORKSPACE if it is not defined. This is useful
// when running tests on the command line and not through Bazel.
//
// Deprecated: With Bzlmod enabled, the workspace name is always "_main".  Use
// [github.com/bazelbuild/rules_go/go/runfiles] instead to access runfiles.
func SetDefaultTestWorkspace(w string) {
	ensureRunfiles()
	runfiles.workspace = w
}

// RunfilesPath return the path to the runfiles tree.
// It will return an error if there is no runfiles tree, for example because
// the executable is run on Windows or was not invoked with 'bazel test'
// or 'bazel run'.
//
// Deprecated: Use [github.com/bazelbuild/rules_go/go/runfiles] instead to
// access runfiles, which provides a platform-agnostic [fs.FS] implementation.
func RunfilesPath() (string, error) {
	if err := ensureRunfiles(); err != nil {
		return "", err
	}
	if runfiles.dir == "" {
		if runtime.GOOS == "windows" {
			return "", errors.New("RunfilesPath: no runfiles directory on windows")
		} else {
			return "", errors.New("could not locate runfiles directory")
		}
	}
	if runfiles.workspace == "" {
		return "", errors.New("could not locate runfiles workspace")
	}
	return filepath.Join(runfiles.dir, runfiles.workspace), nil
}

var runfiles = struct {
	once, listOnce sync.Once

	// list is a list of known runfiles, either loaded from the manifest
	// or discovered by walking the runfile directory.
	list []RunfileEntry

	// index maps runfile short paths to absolute paths.
	index index

	// dir is a path to the runfile directory. Typically this is a directory
	// named <target>.runfiles, with a subdirectory for each workspace.
	dir string

	// workspace is workspace where the binary or test was built.
	workspace string

	// workspaces is a list of other workspace names.
	workspaces []string

	// err is set when there is an error loading runfiles, for example,
	// parsing the manifest.
	err error
}{}

type index struct {
	indexWithWorkspace     map[indexKey]*RunfileEntry
	indexIgnoringWorksapce map[string]*RunfileEntry
}

func newIndex() index {
	return index{
		indexWithWorkspace:     make(map[indexKey]*RunfileEntry),
		indexIgnoringWorksapce: make(map[string]*RunfileEntry),
	}
}

func (i *index) Put(entry *RunfileEntry) {
	i.indexWithWorkspace[indexKey{
		workspace: entry.Workspace,
		shortPath: entry.ShortPath,
	}] = entry
	i.indexIgnoringWorksapce[entry.ShortPath] = entry
}

func (i *index) Get(workspace string, shortPath string) string {
	entry := i.indexWithWorkspace[indexKey{
		workspace: workspace,
		shortPath: shortPath,
	}]
	if entry == nil {
		return ""
	}
	return entry.Path
}

func (i *index) GetIgnoringWorkspace(shortPath string) (*RunfileEntry, bool) {
	entry, ok := i.indexIgnoringWorksapce[shortPath]
	return entry, ok
}

type indexKey struct {
	workspace string
	shortPath string
}

func ensureRunfiles() error {
	runfiles.once.Do(initRunfiles)
	return runfiles.err
}

func initRunfiles() {
	manifest := os.Getenv("RUNFILES_MANIFEST_FILE")
	if manifest != "" {
		// On Windows, Bazel doesn't create a symlink tree of runfiles because
		// Windows doesn't support symbolic links by default. Instead, runfile
		// locations are written to a manifest file.
		runfiles.index = newIndex()
		data, err := ioutil.ReadFile(manifest)
		if err != nil {
			runfiles.err = err
			return
		}
		lineno := 0
		for len(data) > 0 {
			i := bytes.IndexByte(data, '\n')
			var line []byte
			if i < 0 {
				line = data
				data = nil
			} else {
				line = data[:i]
				data = data[i+1:]
			}
			lineno++

			// Only TrimRight newlines. Do not TrimRight() completely, because that would remove spaces too.
			// This is necessary in order to have at least one space in every manifest line.
			// Some manifest entries don't have any path after this space, namely the "__init__.py" entries.
			// original comment sourced from: https://github.com/bazelbuild/bazel/blob/09c621e4cf5b968f4c6cdf905ab142d5961f9ddc/src/test/py/bazel/runfiles_test.py#L225
			line = bytes.TrimRight(line, "\r\n")
			if len(line) == 0 {
				continue
			}

			spaceIndex := bytes.IndexByte(line, ' ')
			if spaceIndex < 0 {
				runfiles.err = fmt.Errorf(
					"error parsing runfiles manifest: %s:%d: no space: '%s'", manifest, lineno, line)
				return
			}
			shortPath := string(line[0:spaceIndex])
			abspath := ""
			if len(line) > spaceIndex+1 {
				abspath = string(line[spaceIndex+1:])
			}

			entry := RunfileEntry{ShortPath: shortPath, Path: abspath}
			if i := strings.IndexByte(entry.ShortPath, '/'); i >= 0 {
				entry.Workspace = entry.ShortPath[:i]
				entry.ShortPath = entry.ShortPath[i+1:]
			}
			if strings.HasPrefix(entry.ShortPath, "external/") {
				entry.ShortPath = entry.ShortPath[len("external/"):]
				if i := strings.IndexByte(entry.ShortPath, '/'); i >= 0 {
					entry.Workspace = entry.ShortPath[:i]
					entry.ShortPath = entry.ShortPath[i+1:]
				}
			}
			if strings.HasPrefix(entry.ShortPath, "../") {
				entry.ShortPath = entry.ShortPath[len("../"):]
				if i := strings.IndexByte(entry.ShortPath, '/'); i >= 0 {
					entry.Workspace = entry.ShortPath[:i]
					entry.ShortPath = entry.ShortPath[i+1:]
				}
			}

			runfiles.list = append(runfiles.list, entry)
			runfiles.index.Put(&entry)
		}
	}

	runfiles.workspace = os.Getenv("TEST_WORKSPACE")

	if dir := os.Getenv("RUNFILES_DIR"); dir != "" {
		runfiles.dir = dir
	} else if dir = os.Getenv("TEST_SRCDIR"); dir != "" {
		runfiles.dir = dir
	} else if runtime.GOOS != "windows" {
		dir, err := os.Getwd()
		if err != nil {
			runfiles.err = fmt.Errorf("error locating runfiles dir: %v", err)
			return
		}

		parent := filepath.Dir(dir)
		if strings.HasSuffix(parent, ".runfiles") {
			runfiles.dir = parent
			if runfiles.workspace == "" {
				runfiles.workspace = filepath.Base(dir)
			}
		} else {
			runfiles.err = errors.New("could not locate runfiles directory")
			return
		}
	}

	if runfiles.dir != "" {
		fis, err := ioutil.ReadDir(runfiles.dir)
		if err != nil {
			runfiles.err = fmt.Errorf("could not open runfiles directory: %v", err)
			return
		}
		for _, fi := range fis {
			if fi.IsDir() {
				runfiles.workspaces = append(runfiles.workspaces, fi.Name())
			}
		}
		sort.Strings(runfiles.workspaces)
	}
}
