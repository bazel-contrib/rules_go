// Copyright 2025 The Bazel Authors. All rights reserved.
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

package trimpath_test

import (
	"testing"

	"github.com/bazelbuild/rules_go/go/tools/bazel_testing"
)

func TestMain(m *testing.M) {
	bazel_testing.TestMain(m, bazel_testing.Args{
		Main: `
-- BUILD.bazel --
load("@io_bazel_rules_go//go:def.bzl", "go_binary", "go_library")

go_library(
    name = "maincgo",
    srcs = ["maincgo.go"],
    cgo = True,
    importpath = "example.com/main_repo/maincgo",
    visibility = ["//visibility:private"],
)

go_library(
    name = "main_lib",
    srcs = ["main.go"],
    deps = [":maincgo", "@other_repo//:other", "@other_repo//:othercgo"],
    importpath = "example.com/main_repo/main",
    visibility = ["//visibility:private"],
)

go_binary(
    name = "main",
    embed = [":main_lib"],
    visibility = ["//visibility:public"],
)
-- main.go --
package main

import "runtime"
import "fmt"
import "example.com/main_repo/maincgo"
import "example.com/other_repo/other"
import "example.com/other_repo/othercgo"

func File() string {
	_, file, _, _ := runtime.Caller(0)
	return file
}

func main() {
	fmt.Println(File())
	fmt.Println(maincgo.File())
	fmt.Println(other.File())
	fmt.Println(othercgo.File())
}
-- maincgo.go --
package maincgo

import "C"
import "runtime"

func File() string {
	_, file, _, _ := runtime.Caller(0)
	return file
}
-- other_repo/WORKSPACE --
-- other_repo/BUILD.bazel --
load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "other",
    srcs = ["other.go"],
    importpath = "example.com/other_repo/other",
    visibility = ["//visibility:public"],
)

go_library(
    name = "othercgo",
    srcs = ["othercgo.go"],
    cgo = True,
    importpath = "example.com/other_repo/othercgo",
    visibility = ["//visibility:public"],
)
-- other_repo/other.go --
package other

import "runtime"

func File() string {
	_, file, _, _ := runtime.Caller(0)
	return file
}
-- other_repo/othercgo.go --
package othercgo

import "C"
import "runtime"

func File() string {
	_, file, _, _ := runtime.Caller(0)
	return file
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

// These are the expected paths after applying trimpath.
var expectedDefault = `
main.go
maincgo.go
external/other_repo/other.go
external/other_repo/othercgo.go
`

var expectedSibling = `
main.go
maincgo.go
../other_repo/other.go
../other_repo/othercgo.go
`

func TestTrimpath(t *testing.T) {
	t.Run("default", func(t *testing.T) {
		out, err := bazel_testing.BazelOutput("run", "//:main")
		if err != nil {
			t.Fatal(err)
		}
		outStr := "\n" + string(out)
		if outStr != expectedDefault {
			t.Fatal("actual", outStr, "vs expected", expectedDefault)
		}
	})
	t.Run("experimental_sibling_repository_layout", func(t *testing.T) {
		out, err := bazel_testing.BazelOutput("run", "--experimental_sibling_repository_layout", "//:main")
		if err != nil {
			t.Fatal(err)
		}
		outStr := "\n" + string(out)
		if outStr != expectedSibling {
			t.Fatal("actual", outStr, "vs expected", expectedSibling)
		}
	})
}
