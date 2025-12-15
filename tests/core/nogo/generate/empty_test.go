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

package empty_test

import (
	"testing"

	"github.com/bazelbuild/rules_go/go/tools/bazel_testing"
)

func TestMain(m *testing.M) {
	bazel_testing.TestMain(m, bazel_testing.Args{
		Main: `
-- BUILD.bazel --
load("@io_bazel_rules_go//go:def.bzl", "go_library", "go_test", "nogo")

go_test(
    name = "simple_test",
    size = "small",
    srcs = ["simple_test.go"],
)

nogo(
    name = "nogo",
    deps = ["//noempty"],
    visibility = ["//visibility:public"],
)
-- simple_test.go --
package simple

import (
	"testing"
)

func TestFoo(t *testing.T) {
}
-- noempty/BUILD.bazel --
load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "noempty",
    srcs = ["analyzer.go"],
    importpath = "noempty",
    visibility = ["//visibility:public"],
    deps = [
        "@org_golang_x_tools//go/analysis",
    ],
)
-- noempty/analyzer.go --
package noempty

import (
	"fmt"

	"golang.org/x/tools/go/analysis"
)

var Analyzer = &analysis.Analyzer{
	Name:     "noempty",
	Doc:      "noempty ensure that source code was not a generated file created by rules_go test rewrite",
	Run:      run,
}

func run(pass *analysis.Pass) (interface{}, error) {
	for _, f := range pass.Files {
		// Fail on any package that isn't the "simple_test"'s package ('simple') or 'main'.
		if f.Name.Name != "simple" && f.Name.Name != "main" {
			pass.Report(analysis.Diagnostic{
				Pos:     0,
				Message: fmt.Sprintf("Detected generated source code from rules_go: package %s", f.Name.Name),
			})
		}
	}

	return nil, nil
}
`,
		Nogo: `@//:nogo`,
	})
}

func TestNogoGenEmptyCode(t *testing.T) {
	if out, err := bazel_testing.BazelOutput("build", "-k", "//:simple_test"); err != nil {
		println(string(out))
		t.Fatal(err)
	}
}
