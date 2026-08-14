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

package external_includes_test

import (
	"testing"

	"github.com/bazelbuild/rules_go/go/tools/bazel_testing"
)

func TestMain(m *testing.M) {
	bazel_testing.TestMain(m, bazel_testing.Args{
		Main: `
-- other_repo/MODULE.bazel --
module(name = "other_repo")
bazel_dep(name = "rules_go", repo_name = "io_bazel_rules_go")
bazel_dep(name = "rules_cc", version = "0.1.5")
-- other_repo/cc/BUILD.bazel --
load("@rules_cc//cc:cc_binary.bzl", "cc_binary")

cc_binary(
    name = "main",
    srcs = ["main.c"],
    deps = ["//cgo"],
)
-- other_repo/cc/main.c --
#include "cgo/cgo.h"

int main() {}
-- other_repo/cgo/BUILD.bazel --
load("@io_bazel_rules_go//go:def.bzl", "go_binary", "go_library")

go_binary(
    name = "cgo",
    embed = [":cgo_lib"],
    importpath = "example.com/rules_go/cgo",
    linkmode = "c-archive",
    visibility = ["//visibility:public"],
)

go_library(
    name = "cgo_lib",
    srcs = ["cgo.go"],
    cgo = True,
    importpath = "example.com/rules_go/cgo",
    visibility = ["//visibility:private"],
)
-- other_repo/cgo/cgo.go --
package main

import "C"

//export HelloCgo
func HelloCgo() {}

func main() {}
`,
		ModuleFileSuffix: `
bazel_dep(name = "other_repo", version = "0.0.0")
local_path_override(
    module_name = "other_repo",
    path = "other_repo",
)
`,
	})
}

func TestExternalIncludes(t *testing.T) {
	t.Run("default", func(t *testing.T) {
		if err := bazel_testing.RunBazel("build", "@other_repo//cc:main"); err != nil {
			t.Fatalf("Did not expect error:\n%+v", err)
		}
	})
	t.Run("experimental_sibling_repository_layout", func(t *testing.T) {
		if err := bazel_testing.RunBazel("build", "--experimental_sibling_repository_layout", "@other_repo//cc:main"); err != nil {
			t.Fatalf("Did not expect error:\n%+v", err)
		}
	})
}
