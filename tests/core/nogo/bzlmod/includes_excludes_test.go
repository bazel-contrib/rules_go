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

package includes_excludes_test

import (
	"strings"
	"testing"

	"github.com/bazelbuild/rules_go/go/tools/bazel_testing"
)

func TestMain(m *testing.M) {
	bazel_testing.TestMain(m, bazel_testing.Args{
		Nogo: "@io_bazel_rules_go//:tools_nogo",
		Main: `
-- BUILD.bazel --
load("@io_bazel_rules_go//go:def.bzl", "go_library", "nogo", "TOOLS_NOGO")

nogo(
    name = "my_nogo",
    visibility = ["//visibility:public"],
    deps = TOOLS_NOGO,
)

go_library(
    name = "lib",
    srcs = ["lib.go"],
    importpath = "example.com/lib",
)    

-- lib.go --
package lib

func useless() string {
	foo := "original"
	if foo == "original" {
		foo = foo
		return foo
	}
	return foo
}

-- go/BUILD.bazel --
load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "lib",
    srcs = ["lib.go"],
    importpath = "example.com/go/lib",
)    

-- go/lib.go --
package lib

func useless() string {
	foo := "original"
	if foo == "original" {
		foo = foo
		return foo
	}
	return foo
}

-- go/third_party/BUILD.bazel --
load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "lib",
    srcs = ["lib.go"],
    importpath = "example.com/go/third_party/lib",
)    

-- go/third_party/lib.go --
package lib

func useless() string {
	foo := "original"
	if foo == "original" {
		foo = foo
		return foo
	}
	return foo
}
`,
		ModuleFileSuffix: `
go_sdk = use_extension("@io_bazel_rules_go//go:extensions.bzl", "go_sdk")
go_sdk.nogo(
    nogo = "//:my_nogo",
    includes = ["//go:__subpackages__"],
    excludes = ["//go/third_party:__subpackages__"],
)
`,
	})
}

func TestNotIncluded(t *testing.T) {
	if err := bazel_testing.RunBazel("build", "//:lib"); err != nil {
		t.Fatal(err)
	}
}

func TestIncluded(t *testing.T) {
	if err := bazel_testing.RunBazel("build", "//go:lib"); err == nil {
		t.Fatal("Expected build to fail")
	} else if !strings.Contains(err.Error(), "lib.go:6:3: self-assignment of foo to foo (assign)") {
		t.Fatalf("Expected error to contain \"lib.go:6:3: self-assignment of foo to foo (assign)\", got %s", err)
	}
}

func TestExcluded(t *testing.T) {
	if err := bazel_testing.RunBazel("build", "//go/third_party:lib"); err != nil {
		t.Fatal(err)
	}
}
