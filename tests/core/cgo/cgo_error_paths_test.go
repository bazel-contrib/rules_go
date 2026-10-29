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

package cgo_error_paths_test

import (
	"regexp"
	"strings"
	"testing"

	"github.com/bazelbuild/rules_go/go/tools/bazel_testing"
)

func TestMain(m *testing.M) {
	bazel_testing.TestMain(m, bazel_testing.Args{
		Main: `
-- plain/BUILD.bazel --
load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "plain",
    srcs = [
        "c.go",
        "plain.go",
        "sub/plain.go",
    ],
    cgo = True,
    importpath = "example.com/plain",
)

-- plain/c.go --
package plain

// int two() { return 2; }
import "C"

func Two() int { return int(C.two()) }

-- plain/plain.go --
package plain

func One() int {
	unusedInPlainFile := 1
	return 1
}

-- plain/sub/plain.go --
package plain

func Three() int {
	unusedInSubdirFile := 1
	return 3
}

-- cgofile/BUILD.bazel --
load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "cgofile",
    srcs = ["c.go"],
    cgo = True,
    importpath = "example.com/cgofile",
)

-- cgofile/c.go --
package cgofile

// int two() { return 2; }
import "C"

func Two() int {
	unusedInCgoFile := 1
	return int(C.two())
}
`,
	})
}

// TestCompileErrorPaths checks that compiler errors in cgo packages name the
// packages' source files, not the copies the builder compiles in its temporary
// work directory, which no longer exists once the action has failed.
func TestCompileErrorPaths(t *testing.T) {
	_, stderrBytes, err := bazel_testing.BazelOutputWithInput(nil, "build", "--keep_going", "//...")
	if err == nil {
		t.Fatal("build succeeded, want compile errors")
	}
	stderr := string(stderrBytes)

	for _, want := range []string{
		// A plain Go file in a cgo package.
		"plain/plain.go:4:2: declared and not used: unusedInPlainFile",
		// A file whose base name the builder had to change when copying it.
		"plain/sub/plain.go:4:2: declared and not used: unusedInSubdirFile",
		// A file processed by cgo, whose //line directives the compiler resolves
		// relative to the work directory.
		"cgofile/c.go:7:2: declared and not used: unusedInCgoFile",
	} {
		// The compiler separates path elements with the host's separator.
		pattern := strings.ReplaceAll(regexp.QuoteMeta(want), "/", `[/\\]`)
		if !regexp.MustCompile(`(?m)^` + pattern + `$`).MatchString(stderr) {
			t.Errorf("compile errors do not contain the line %q", want)
		}
	}
	if strings.Contains(stderr, "rules_go_work") {
		t.Error("compile errors name files in the builder's temporary work directory")
	}
	if t.Failed() {
		t.Log(stderr)
	}
}
