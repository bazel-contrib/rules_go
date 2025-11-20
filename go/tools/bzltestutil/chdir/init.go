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

// Package chdir provides an init function that changes the current working
// directory to RunDir when the test executable is started by Bazel
// (when TEST_SRCDIR and TEST_WORKSPACE are set).
//
// This hides a difference between Bazel and 'go test': 'go test' starts test
// executables in the package source directory, while Bazel starts test
// executables in a directory made to look like the repository root directory.
// Tests frequently refer to testdata files using paths relative to their
// package directory, so open source tests frequently break unless they're
// written with Bazel specifically in mind (using go/runfiles).
//
// For this init function to work, it must be called before init functions
// in all user packages.
//
// In Go 1.20 and earlier, the package initialization order was underspecified,
// other than a requirement that each package is initialized after all its
// transitively imported packages. We relied on the linker initializing
// packages in the order their imports appeared in source, so we import
// bzltestutil (and transitively, this package) from the generated test main
// before other packages.
//
// In Go 1.21, the package initialization order was clarified, and the
// linker implementation was changed. See
// https://go.dev/ref/spec#Program_initialization or
// https://go.dev/doc/go1.21#language.
//
// > Given the list of all packages, sorted by import path, in each step the
// > first uninitialized package in the list for which all imported packages
// > (if any) are already initialized is initialized. This step is repeated
// > until all packages are initialized.
//
// To ensure this package is initialized before user code without injecting
// edges into the dependency graph, we implement the following hack:
//
// 1. Add the prefix '+initfirst/' to this package's path with the 'importmap'
//    attribute. '+' is the first allowed character that sorts higher than
//    letters. Because we're using 'importmap' and not 'importpath', this
//    package may be imported in .go files without the prefix.
// 2. Put this init function in a separate package that only imports "os".
//    Previously, this function was in bzltestutil, but bzltest util imports
//    several other std packages may be get initialized later. For example,
//    the "sync" package is initialized after a user package named
//    "github.com/a/b" that only imports "os", and because bzltestutil imports
//    "sync", it would get initialized even later. A user package that imports
//    nothing may still be initialized before "os", but we assume "os"
//    is needed to observe the current directory.
package chdir

// This package should import nothing other than "os"
// and packages imported by "os" (run 'go list -deps os').
import "os"

var (
	// Initialized by linker.
	RunDir string

	// Initial working directory.
	TestExecDir string
)

const isWindows = os.PathSeparator == '\\'

func init() {
	if os.Getenv("GO_TEST_RUN_FROM_BAZEL") == "" {
		return
	}

	var err error
	TestExecDir, err = os.Getwd()
	if err != nil {
		panic(err)
	}

	// Check if we're being run by Bazel and change directories if so.
	// TEST_SRCDIR and TEST_WORKSPACE are set by the Bazel test runner, so that makes a decent proxy.
	testSrcDir, hasSrcDir := os.LookupEnv("TEST_SRCDIR")
	testWorkspace, hasWorkspace := os.LookupEnv("TEST_WORKSPACE")
	if hasSrcDir && hasWorkspace && RunDir != "" {
		abs := RunDir
		if !filepathIsAbs(RunDir) {
			abs = filepathJoin(testSrcDir, testWorkspace, RunDir)
		}
		err := os.Chdir(abs)
		// Ignore the Chdir err when on Windows, since it might have have runfiles symlinks.
		// https://github.com/bazelbuild/rules_go/pull/1721#issuecomment-422145904
		if err != nil && !isWindows {
			panic("could not change to test directory: " + err.Error())
		}
		if err == nil {
			os.Setenv("PWD", abs)
		}
	}
}

// filepathIsAbs is a primitive version of filepath.IsAbs. It handles the
// cases we are likely to encounter but is not specialized at compile time
// and does not support DOS device paths (\\.\UNC\host\share\...) nor
// Plan 9 absolute paths (starting with #).
func filepathIsAbs(path string) bool {
	if isWindows {
		// Drive-letter path
		if len(path) >= 3 &&
			('A' <= path[0] && path[0] <= 'Z' || 'a' <= path[0] && path[0] <= 'z') &&
			path[1] == ':' &&
			(path[2] == '\\' || path[2] == '/') {
			return true
		}

		// UNC path
		if len(path) >= 2 && path[0] == '\\' && path[1] == '\\' {
			return true
		}
		return false
	}

	return len(path) > 0 && path[0] == '/'
}

// filepathJoin is a primitive version of filepath.Join. It only joins
// its arguments with os.PathSeparator. It does not clean arguments first.
func filepathJoin(base string, parts ...string) string {
	n := len(base)
	for _, part := range parts {
		n += 1 + len(part)
	}
	buf := make([]byte, 0, n)
	buf = append(buf, []byte(base)...)
	for _, part := range parts {
		buf = append(buf, os.PathSeparator)
		buf = append(buf, []byte(part)...)
	}
	return string(buf)
}
