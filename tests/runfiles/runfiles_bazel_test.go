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

package runfiles_test

import (
	"regexp"
	"testing"

	"github.com/bazelbuild/rules_go/go/tools/bazel_testing"
)

func TestMain(m *testing.M) {
	bazel_testing.TestMain(m, bazel_testing.Args{
		Main: `
-- other_repo/WORKSPACE --
-- other_repo/pkg/BUILD.bazel --
load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "external_source_lib",
    srcs = ["external_source_lib.go"],
    importpath = "example.com/runfiles/external_source_lib",
    deps = [
        "@io_bazel_rules_go//go/runfiles",
    ],
    visibility = ["//visibility:public"],
)

genrule(
    name = "gen_source",
    srcs = ["external_source_lib.go"],
    outs = ["external_generated_lib.go"],
    cmd = "cat $(location external_source_lib.go) | sed 's/external_source_lib/external_generated_lib/g' > $@",
)

go_library(
    name = "external_generated_lib",
    srcs = [":gen_source"],
    importpath = "example.com/runfiles/external_generated_lib",
    deps = [
        "@io_bazel_rules_go//go/runfiles",
    ],
    visibility = ["//visibility:public"],
)

# The recorded source paths of this library are relative to the repository
# root rather than the execroot, so the repository name cannot be recovered
# from them and CurrentRepository has to consult the package repository map.
go_library(
    name = "external_trimmed_lib",
    srcs = ["external_trimmed_lib.go"],
    gc_goopts = ["-trimpath=external"],
    importpath = "example.com/runfiles/external_trimmed_lib",
    deps = [
        "@io_bazel_rules_go//go/runfiles",
    ],
    visibility = ["//visibility:public"],
)
-- other_repo/pkg/external_trimmed_lib.go --
package external_trimmed_lib

import (
	"fmt"
	"runtime"

	"github.com/bazelbuild/rules_go/go/runfiles"
)

func PrintRepo() {
	_, file, _, _ := runtime.Caller(0)
	fmt.Printf("%s: '%s'\n", file, runfiles.CurrentRepository())
}
-- other_repo/pkg/external_source_lib.go --
package external_source_lib

import (
	"fmt"
	"runtime"

	"github.com/bazelbuild/rules_go/go/runfiles"
)

func PrintRepo() {
	_, file, _, _ := runtime.Caller(0)
	fmt.Printf("%s: '%s'\n", file, runfiles.CurrentRepository())
}
-- pkg/BUILD.bazel --
-- pkg/BUILD.bazel --
load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "internal_source_lib",
    srcs = ["internal_source_lib.go"],
    importpath = "example.com/runfiles/internal_source_lib",
    deps = [
        "@io_bazel_rules_go//go/runfiles",
    ],
    visibility = ["//visibility:public"],
)

genrule(
    name = "gen_source",
    srcs = ["internal_source_lib.go"],
    outs = ["internal_generated_lib.go"],
    cmd = "cat $(location internal_source_lib.go) | sed 's/internal_source_lib/internal_generated_lib/g' > $@",
)

go_library(
    name = "internal_generated_lib",
    srcs = [":gen_source"],
    importpath = "example.com/runfiles/internal_generated_lib",
    deps = [
        "@io_bazel_rules_go//go/runfiles",
    ],
    visibility = ["//visibility:public"],
)
-- pkg/internal_source_lib.go --
package internal_source_lib

import (
	"fmt"
	"runtime"

	"github.com/bazelbuild/rules_go/go/runfiles"
)

func PrintRepo() {
	_, file, _, _ := runtime.Caller(0)
	fmt.Printf("%s: '%s'\n", file, runfiles.CurrentRepository())
}
-- BUILD.bazel --
load("@io_bazel_rules_go//go:def.bzl", "go_binary")

go_binary(
    name = "main",
    srcs = ["main.go"],
    deps = [
        "//pkg:internal_source_lib",
        "//pkg:internal_generated_lib",
        "@other_repo//pkg:external_source_lib",
        "@other_repo//pkg:external_generated_lib",
        "@other_repo//pkg:external_trimmed_lib",
    ],
)
-- main.go --
package main

import (
	"example.com/runfiles/internal_generated_lib"
	"example.com/runfiles/internal_source_lib"
	"example.com/runfiles/external_generated_lib"
	"example.com/runfiles/external_source_lib"
	"example.com/runfiles/external_trimmed_lib"
)

func main() {
	internal_source_lib.PrintRepo()
	internal_generated_lib.PrintRepo()
	external_source_lib.PrintRepo()
	external_generated_lib.PrintRepo()
	external_trimmed_lib.PrintRepo()
}
`,
		WorkspaceSuffix: `
local_repository(
    name = "other_repo",
    path = "other_repo",
)
`,
	})
}

var expectedOutput = regexp.MustCompile(`^pkg/internal_source_lib.go: ''
bazel-out/[^/]+/bin/pkg/internal_generated_lib.go: ''
external/other_repo/pkg/external_source_lib.go: 'other_repo'
bazel-out/[^/]+/bin/external/other_repo/pkg/external_generated_lib.go: 'other_repo'
other_repo/pkg/external_trimmed_lib.go: 'other_repo'
$`)

func TestCurrentRepository(t *testing.T) {
	out, err := bazel_testing.BazelOutput("run", "//:main")
	if err != nil {
		t.Fatal(err)
	}
	if !expectedOutput.Match(out) {
		t.Fatalf("got: %q, want: %q", string(out), expectedOutput.String())
	}
}
