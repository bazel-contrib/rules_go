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

package suites_test

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
    deps = [":foofuncname"],
    suites = {":suite": "Analyzers"},
    visibility = ["//visibility:public"],
)

go_library(
    name = "foofuncname",
    srcs = ["foofuncname.go"],
    importpath = "foofuncname",
    deps = ["@org_golang_x_tools//go/analysis"],
)

go_library(
    name = "suite",
    srcs = ["suite.go"],
    importpath = "suite",
    deps = [
        ":foofuncname",
        "@org_golang_x_tools//go/analysis",
    ],
)

go_library(
    name = "has_errors",
    srcs = ["has_errors.go"],
    importpath = "haserrors",
)

-- foofuncname.go --
package foofuncname

import (
	"go/ast"

	"golang.org/x/tools/go/analysis"
)

var Analyzer = &analysis.Analyzer{
	Name: "foofuncname",
	Doc:  "reports functions named Foo",
	Run: func(pass *analysis.Pass) (interface{}, error) {
		for _, f := range pass.Files {
			for _, d := range f.Decls {
				if fn, ok := d.(*ast.FuncDecl); ok && fn.Name.Name == "Foo" {
					pass.Reportf(fn.Pos(), "function must not be named Foo")
				}
			}
		}
		return nil, nil
	},
}

-- suite.go --
package suite

import (
	"foofuncname"

	"golang.org/x/tools/go/analysis"
)

var Analyzers = []*analysis.Analyzer{foofuncname.Analyzer, importfmt}

var importfmt = &analysis.Analyzer{
	Name: "importfmt",
	Doc:  "reports imports of fmt",
	Run: func(pass *analysis.Pass) (interface{}, error) {
		for _, f := range pass.Files {
			for _, imp := range f.Imports {
				if imp.Path.Value == ` + "`" + `"fmt"` + "`" + ` {
					pass.Reportf(imp.Pos(), "package fmt must not be imported")
				}
			}
		}
		return nil, nil
	},
}

-- has_errors.go --
package haserrors

import "fmt"

func Foo() { fmt.Println() }
`,
	})
}

// Both analyzers in the suite report, and foofuncname, which is also in deps,
// reports once.
func TestSuites(t *testing.T) {
	err := bazel_testing.RunBazel("build", "//:has_errors")
	if err == nil {
		t.Fatal("expected the build to fail on the suite's findings")
	}
	out := err.Error()
	for _, want := range []string{
		"package fmt must not be imported (importfmt)",
		"function must not be named Foo (foofuncname)",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("expected the build error to contain %q, got:\n%s", want, out)
		}
	}
	if n := strings.Count(out, "function must not be named Foo (foofuncname)"); n != 1 {
		t.Errorf("expected foofuncname to report once, got %d:\n%s", n, out)
	}
}
