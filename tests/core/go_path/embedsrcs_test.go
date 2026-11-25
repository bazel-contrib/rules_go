// Copyright 2026 The Bazel Authors. All rights reserved.
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

package embedsrcs_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/bazelbuild/rules_go/go/tools/bazel_testing"
)

func TestMain(m *testing.M) {
	bazel_testing.TestMain(m, bazel_testing.Args{
		Main: `
-- BUILD.bazel --
load("@io_bazel_rules_go//go:def.bzl", "go_path")

go_path(
    name = "gopath",
    mode = "copy",
    deps = ["//lib"],
)

-- lib/BUILD.bazel --
load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "lib",
    srcs = ["lib.go"],
    embedsrcs = ["note/a.txt"],
    importpath = "example.com/lib",
    visibility = ["//visibility:public"],
)

-- lib/lib.go --
package lib

import _ "embed"

//go:embed note/a.txt
var Note string

-- lib/note/a.txt --
note
`,
	})
}

// TestEmbedsrcKeepsItsPathInPackage checks that an embedded file is placed at its
// path relative to the package. The package directory "lib" and the file's
// directory "note" use only characters found in every Bazel output directory
// path, so removing those characters instead of the output directory prefix
// would place the file at src/example.com/lib/.txt.
func TestEmbedsrcKeepsItsPathInPackage(t *testing.T) {
	if err := bazel_testing.RunBazel("build", "//:gopath"); err != nil {
		t.Fatal(err)
	}
	want := filepath.Join("bazel-bin", "gopath", "src", "example.com", "lib", "note", "a.txt")
	if _, err := os.Stat(want); err != nil {
		var got []string
		filepath.Walk(filepath.Join("bazel-bin", "gopath", "src"), func(path string, info os.FileInfo, err error) error {
			if err == nil && !info.IsDir() {
				got = append(got, path)
			}
			return nil
		})
		t.Fatalf("embedded file not found at %s; go_path contains %v", want, got)
	}
}
