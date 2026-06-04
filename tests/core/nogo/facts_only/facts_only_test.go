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

package facts_only_test

import (
	"strings"
	"testing"

	"github.com/bazelbuild/rules_go/go/tools/bazel_testing"
)

// Packages outside the nogo includes are compiled with -facts_only:
// diagnostics are discarded, but facts must still be produced for the
// packages that import them. The usemarked analyzer below declares no
// fact types of its own: the facts belong to the marker analyzer it
// requires, which exposes them to usemarked as its result (lostcancel
// and ctrlflow have the same relationship in the default analyzer
// set). These tests cover that the fact producers required by such an
// analyzer still run in facts-only compiles, even though the analyzer
// itself has nothing to contribute there.
func TestMain(m *testing.M) {
	bazel_testing.TestMain(m, bazel_testing.Args{
		Nogo:         "@//:nogo",
		NogoIncludes: []string{"@//use:__subpackages__"},
		Main: `
-- BUILD.bazel --
load("@io_bazel_rules_go//go:def.bzl", "go_library", "nogo")

nogo(
    name = "nogo",
    deps = [":usemarked"],
    visibility = ["//visibility:public"],
)

go_library(
    name = "marker",
    srcs = ["marker.go"],
    importpath = "markeranalyzer",
    deps = ["@org_golang_x_tools//go/analysis"],
    visibility = ["//visibility:public"],
)

go_library(
    name = "usemarked",
    srcs = ["usemarked.go"],
    importpath = "usemarkedanalyzer",
    deps = [
        ":marker",
        "@org_golang_x_tools//go/analysis",
    ],
    visibility = ["//visibility:public"],
)

-- marker.go --
// Package marker exports a fact for every function whose name starts
// with "Marked" and returns the set of marked objects visible to the
// package being analyzed, including objects from its imports. It also
// reports a diagnostic at every marked declaration, so tests can tell
// that diagnostics from facts-only compiles are discarded even though
// the analyzer ran.
package marker

import (
	"go/ast"
	"go/types"
	"reflect"
	"strings"

	"golang.org/x/tools/go/analysis"
)

type IsMarked struct{}

func (*IsMarked) AFact() {}

var Analyzer = &analysis.Analyzer{
	Name:       "marker",
	Doc:        "exports a fact for functions whose name starts with Marked",
	Run:        run,
	FactTypes:  []analysis.Fact{(*IsMarked)(nil)},
	ResultType: reflect.TypeOf(map[types.Object]bool(nil)),
}

func run(pass *analysis.Pass) (interface{}, error) {
	for _, f := range pass.Files {
		for _, decl := range f.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok || !strings.HasPrefix(fn.Name.Name, "Marked") {
				continue
			}
			if obj := pass.TypesInfo.Defs[fn.Name]; obj != nil {
				pass.ExportObjectFact(obj, &IsMarked{})
				pass.Reportf(fn.Pos(), "declaration of marked function %s", fn.Name.Name)
			}
		}
	}
	marked := make(map[types.Object]bool)
	for _, fact := range pass.AllObjectFacts() {
		marked[fact.Object] = true
	}
	return marked, nil
}

-- usemarked.go --
// Package usemarked reports calls to functions the marker analyzer
// marked. It declares no fact types of its own: the facts are
// produced and read by the required marker analyzer, so usemarked
// only works if marker also ran on the packages that declare the
// called functions.
package usemarked

import (
	"go/ast"
	"go/types"

	"golang.org/x/tools/go/analysis"

	"markeranalyzer"
)

var Analyzer = &analysis.Analyzer{
	Name:     "usemarked",
	Doc:      "reports calls to functions marked by the marker analyzer",
	Run:      run,
	Requires: []*analysis.Analyzer{marker.Analyzer},
}

func run(pass *analysis.Pass) (interface{}, error) {
	marked := pass.ResultOf[marker.Analyzer].(map[types.Object]bool)
	for _, f := range pass.Files {
		ast.Inspect(f, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}
			var id *ast.Ident
			switch fun := call.Fun.(type) {
			case *ast.Ident:
				id = fun
			case *ast.SelectorExpr:
				id = fun.Sel
			default:
				return true
			}
			if obj := pass.TypesInfo.Uses[id]; obj != nil && marked[obj] {
				pass.Reportf(call.Pos(), "call to marked function %s", obj.Name())
			}
			return true
		})
	}
	return nil, nil
}

-- dep/BUILD.bazel --
load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "dep",
    srcs = ["dep.go"],
    importpath = "example.com/dep",
    visibility = ["//visibility:public"],
)

-- dep/dep.go --
package dep

func MarkedDoNotUse() int { return 1 }

func helper() int { return MarkedDoNotUse() }

var _ = helper

-- use/BUILD.bazel --
load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "use",
    srcs = ["use.go"],
    importpath = "example.com/use",
    deps = ["//dep"],
)

-- use/use.go --
package use

import "example.com/dep"

func F() int { return dep.MarkedDoNotUse() }
`,
	})
}

// TestFactsFromUncheckedDependency builds a package inside the nogo
// includes that calls a marked function declared in a package outside
// them. The dependency is compiled with -facts_only, and the fact it
// exports must still reach the dependent's check.
func TestFactsFromUncheckedDependency(t *testing.T) {
	err := bazel_testing.RunBazel("build", "//use")
	if err == nil {
		t.Fatal("expected build of //use to fail with a usemarked diagnostic")
	}
	if want := "call to marked function MarkedDoNotUse (usemarked)"; !strings.Contains(err.Error(), want) {
		t.Fatalf("expected error to contain %q, got:\n%s", want, err)
	}
	if leaked := "declaration of marked function"; strings.Contains(err.Error(), leaked) {
		t.Fatalf("diagnostics from the dependency's facts-only compile leaked into the dependent's validation:\n%s", err)
	}
}

// TestNoDiagnosticsOutsideIncludes builds the dependency directly: it
// is outside the nogo includes, declares a marked function (which the
// marker analyzer that runs for facts also reports on), and calls it.
// The facts-only compile must discard those diagnostics and succeed.
func TestNoDiagnosticsOutsideIncludes(t *testing.T) {
	if err := bazel_testing.RunBazel("build", "//dep"); err != nil {
		t.Fatal(err)
	}
}
