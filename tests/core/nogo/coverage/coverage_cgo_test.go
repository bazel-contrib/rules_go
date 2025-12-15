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

package coverage_cgo_test

import (
	"testing"

	"github.com/bazelbuild/rules_go/go/tools/bazel_testing"
)

func TestMain(m *testing.M) {
	bazel_testing.TestMain(m, bazel_testing.Args{
		Main: `
-- BUILD.bazel --
load("@io_bazel_rules_go//go:def.bzl", "go_library", "go_test", "nogo")

go_library(
    name = "foo_cgo",
    cgo = True,
    srcs = ["foo_cgo.go"],
    importpath = "foo_cgo"
)

go_test(
    name = "foo_cgo_test",
    srcs = ["foo_cgo_test.go"],
    embed = [":foo_cgo"]
)

nogo(
    name = "nogo",
    deps = ["//noinit"],
    visibility = ["//visibility:public"],
)
-- foo_cgo.go --
package foo_cgo

import "C"

func FooCgo() string {
	return "foo_cgo"
}
-- foo_cgo_test.go --
package foo_cgo

import "testing"

func TestFooCgo(t *testing.T) {
	if actual, expected := FooCgo(), "foo_cgo"; actual != expected {
		t.Errorf("FooCgo() should return foo_cgo")
	}
}
-- noinit/BUILD.bazel --
load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "noinit",
    srcs = ["analyzer.go"],
    importpath = "noinit",
    visibility = ["//visibility:public"],
    deps = [
        "@org_golang_x_tools//go/analysis",
    ],
)
-- noinit/analyzer.go --
package noinit
import (
	"fmt"
	"go/ast"

	"golang.org/x/tools/go/analysis"
)

const Name = "gochecknoinits"

var Analyzer = &analysis.Analyzer{
	Name: Name,
	Doc:  "Checks that no init() functions are present in Go code",
	Run:  run,
}

func run(pass *analysis.Pass) (interface{}, error) {
	for _, f := range pass.Files {
		for _, decl := range f.Decls {
			funcDecl, ok := decl.(*ast.FuncDecl)
			if !ok {
				continue
			}
			name := funcDecl.Name.Name
			if name == "init" && funcDecl.Recv.NumFields() == 0 {
				pass.Report(analysis.Diagnostic{
					Pos:      funcDecl.Pos(),
					Message:  fmt.Sprintf("don't use init function"),
					Category: Name,
				})
			}
		}
	}

	return nil, nil
}
`,
		Nogo: `@//:nogo`,
	})
}

func TestNogoWithCoverageAndCgo(t *testing.T) {
	if out, err := bazel_testing.BazelOutput("coverage", "//:foo_cgo_test"); err != nil {
		println(string(out))
		t.Fatal(err)
	}
}
