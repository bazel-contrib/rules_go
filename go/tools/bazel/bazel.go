// Copyright 2017 Google Inc.
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

// Package bazel provides utilities for interacting with the surrounding Bazel environment.
//
// All functions other than NewTmpDir and TestTmpDir are deprecated. Use
// github.com/bazelbuild/rules_go/go/runfiles instead to access runfiles.
package bazel

import (
	"fmt"
	"io/ioutil"
	"os"
)

// The name of the environment variable to access test data files.
//
// Deprecated: Use [github.com/bazelbuild/rules_go/go/runfiles] instead to
// access runfiles.
const TEST_SRCDIR = "TEST_SRCDIR"

const TEST_TMPDIR = "TEST_TMPDIR"

// The name of the environment variable to access the test workspace name.
// With Bzlmod enabled, the corresponding environment variable has the constant
// value "_main".
//
// Deprecated: Use [github.com/bazelbuild/rules_go/go/runfiles] instead to
// access runfiles.
const TEST_WORKSPACE = "TEST_WORKSPACE"

// NewTmpDir creates a new temporary directory in [TestTmpDir].
func NewTmpDir(prefix string) (string, error) {
	return ioutil.TempDir(TestTmpDir(), prefix)
}

// TestTmpDir returns the path the Bazel test temp directory.
// If TEST_TMPDIR is not defined, it returns the OS default temp dir.
func TestTmpDir() string {
	if tmp, ok := os.LookupEnv(TEST_TMPDIR); ok {
		return tmp
	}
	return os.TempDir()
}

// SpliceDelimitedOSArgs is a utility function that scans the [os.Args] list
// for entries delimited by the begin and end delimiters (typically the values
// "-begin_files" and "-end_files" are used).  Entries between these delimiters
// are spliced out of from os.Args and returned to the caller.  If the ordering
// of -begin_files or -end_files is malformed, error is returned.
//
// Deprecated: This method is meant for internal use by bazel_testing only.
func SpliceDelimitedOSArgs(begin, end string) ([]string, error) {
	var files []string
	beginFiles, endFiles := -1, -1
	for i, arg := range os.Args {
		if arg == begin {
			beginFiles = i
		} else if arg == end {
			endFiles = i
			break
		} else if arg == "--" {
			break
		}
	}

	if beginFiles >= 0 && endFiles < 0 ||
		beginFiles < 0 && endFiles >= 0 ||
		beginFiles >= 0 && beginFiles >= endFiles {
		return nil, fmt.Errorf("error: %s, %s not set together or in order", begin, end)
	}

	if beginFiles >= 0 {
		files = os.Args[beginFiles+1 : endFiles]
		os.Args = append(os.Args[:beginFiles:beginFiles], os.Args[endFiles+1:]...)
	}

	return files, nil
}
