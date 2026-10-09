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

package crash_test

import (
	"strings"
	"testing"

	"github.com/bazelbuild/rules_go/go/tools/bazel_testing"
)

func TestMain(m *testing.M) {
	bazel_testing.TestMain(m, bazel_testing.Args{
		Nogo: "@//:nogo",
		Main: `
-- BUILD.bazel --
load("@io_bazel_rules_go//go:def.bzl", "go_library", "nogo")

nogo(
    name = "nogo",
    deps = [":panicky"],
    visibility = ["//visibility:public"],
)

go_library(
    name = "panicky",
    srcs = ["panicky.go"],
    importpath = "panicky",
    deps = ["@org_golang_x_tools//go/analysis"],
)

go_library(
    name = "hello",
    srcs = ["hello.go"],
    importpath = "hello",
)

-- panicky.go --
package panicky

import "golang.org/x/tools/go/analysis"

func init() {
	panic("panicky analyzer failed to initialize")
}

var Analyzer = &analysis.Analyzer{
	Name: "panicky",
	Doc:  "never runs: its package panics at init",
	Run:  func(*analysis.Pass) (interface{}, error) { return nil, nil },
}

-- hello.go --
package hello

func Hello() string { return "hello" }
`,
	})
}

// A panic exits with the same code as a nogo run with findings. The crash must
// fail the build with its output instead of only as a missing .facts output.
func TestCrashIsReported(t *testing.T) {
	err := bazel_testing.RunBazel("build", "//:hello")
	if err == nil {
		t.Fatal("expected the build to fail because the nogo binary panics")
	}
	if want := "panicky analyzer failed to initialize"; !strings.Contains(err.Error(), want) {
		t.Fatalf("expected the build error to contain the panic %q, got:\n%s", want, err)
	}
}
