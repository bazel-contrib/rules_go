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

package custom_test

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"regexp"
	"testing"

	"github.com/bazelbuild/rules_go/go/tools/bazel_testing"
)

const origConfig = `# config = "",`

func TestMain(m *testing.M) {
	bazel_testing.TestMain(m, bazel_testing.Args{
		Nogo: "@//:nogo",
		Main: `
-- BUILD.bazel --
load("@io_bazel_rules_go//go:def.bzl", "go_binary", "go_library", "nogo")

nogo(
    name = "nogo",
    deps = [
        ":foofuncname",
        ":importfmt",
        ":visibility",
        ":cgogen",
        ":panicalyzer",
    ],
    # config = "",
    visibility = ["//visibility:public"],
)

go_library(
    name = "importfmt",
    srcs = ["importfmt.go"],
    importpath = "importfmtanalyzer",
    deps = ["@org_golang_x_tools//go/analysis"],
    visibility = ["//visibility:public"],
)

go_library(
    name = "foofuncname",
    srcs = ["foofuncname.go"],
    importpath = "foofuncanalyzer",
    deps = ["@org_golang_x_tools//go/analysis"],
    visibility = ["//visibility:public"],
)

go_library(
    name = "visibility",
    srcs = ["visibility.go"],
    importpath = "visibilityanalyzer",
    deps = [
        "@org_golang_x_tools//go/analysis",
        "@org_golang_x_tools//go/ast/inspector",
    ],
    visibility = ["//visibility:public"],
)

go_library(
    name = "cgogen",
    srcs = ["cgogen.go"],
    importpath = "cgogenanalyzer",
    deps = ["@org_golang_x_tools//go/analysis"],
    visibility = ["//visibility:public"],
)

go_library(
    name = "panicalyzer",
    srcs = ["panicalyzer.go"],
    importpath = "panicalyzer",
    deps = ["@org_golang_x_tools//go/analysis"],
    visibility = ["//visibility:public"],
)

go_library(
    name = "has_errors",
    srcs = ["has_errors.go"],
    importpath = "haserrors",
    deps = [":dep"],
)

go_library(
    name = "has_errors_linedirective",
    srcs = ["has_errors_linedirective.go"],
    importpath = "haserrors_linedirective",
    deps = [":dep"],
)

go_library(
    name = "uses_cgo_clean",
    srcs = ["examplepkg/uses_cgo_clean.go"],
    importpath = "uses_cgo_clean",
    cgo = True,
)

go_library(
    name = "uses_cgo_with_errors",
    srcs = [
        "examplepkg/uses_cgo_clean.go",
        "examplepkg/pure_src_with_err_calling_native.go",
    ],
    importpath = "examplepkg",
    cgo = True,
)

go_library(
    name = "no_errors",
    srcs = ["no_errors.go"],
    importpath = "noerrors",
    deps = [":dep"],
)

go_library(
    name = "dep",
    srcs = ["dep.go"],
    importpath = "dep",
)

go_binary(
	name = "type_check_fail",
	srcs = ["type_check_fail.go"],
	pure = "on",
)

go_library(
    name = "panics",
    srcs = ["panics.go"],
	importpath = "panics",
)

-- foofuncname.go --
// foofuncname checks for functions named "Foo".
// It has the same package name as another check to test the checks with
// the same package name do not conflict.
package importfmt

import (
	"go/ast"

	"golang.org/x/tools/go/analysis"
)

const doc = "report calls of functions named \"Foo\"\n\nThe foofuncname analyzer reports calls to functions that are\nnamed \"Foo\"."

var Analyzer = &analysis.Analyzer{
	Name: "foofuncname",
	Run:  run,
	Doc:  doc,
}

func run(pass *analysis.Pass) (interface{}, error) {
	for _, f := range pass.Files {
		// TODO(samueltan): use package inspector once the latest golang.org/x/tools
		// changes are pulled into this branch  (see #1755).
		ast.Inspect(f, func(n ast.Node) bool {
			switch n := n.(type) {
			case *ast.FuncDecl:
				if n.Name.Name == "Foo" {
					pass.Reportf(n.Pos(), "function must not be named Foo")
				}
				return true
			}
			return true
		})
	}
	return nil, nil
}

-- importfmt.go --
// importfmt checks for the import of package fmt.
package importfmt

import (
	"go/ast"
	"strconv"

	"golang.org/x/tools/go/analysis"
)

const doc = "report imports of package fmt\n\nThe importfmt analyzer reports imports of package fmt."

var Analyzer = &analysis.Analyzer{
	Name: "importfmt",
	Run:  run,
	Doc:  doc,
}

func run(pass *analysis.Pass) (interface{}, error) {
	for _, f := range pass.Files {
		// TODO(samueltan): use package inspector once the latest golang.org/x/tools
		// changes are pulled into this branch (see #1755).
		ast.Inspect(f, func(n ast.Node) bool {
			switch n := n.(type) {
			case *ast.ImportSpec:
				if path, _ := strconv.Unquote(n.Path.Value); path == "fmt" {
					pass.Reportf(n.Pos(), "package fmt must not be imported")
				}
				return true
			}
			return true
		})
	}
	return nil, nil
}

-- visibility.go --
// visibility looks for visibility annotations on functions and
// checks they are only called from packages allowed to call them.
package visibility

import (
	"encoding/gob"
	"go/ast"
	"regexp"

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/ast/inspector"
)

var Analyzer = &analysis.Analyzer{
	Name: "visibility",
	Run:  run,
	Doc: "enforce visibility requirements for functions\n\nThe visibility analyzer reads visibility annotations on functions and\nchecks that packages that call those functions are allowed to do so.",
	FactTypes: []analysis.Fact{(*VisibilityFact)(nil)},
}

type VisibilityFact struct {
	Paths []string
}

func (_ *VisibilityFact) AFact() {} // dummy method to satisfy interface

func init() { gob.Register((*VisibilityFact)(nil)) }

var visibilityRegexp = regexp.MustCompile("visibility:([^\\s]+)")

func run(pass *analysis.Pass) (interface{}, error) {
	in := inspector.New(pass.Files)

	// Find visibility annotations on function declarations.
	in.Nodes([]ast.Node{(*ast.FuncDecl)(nil)}, func(n ast.Node, push bool) (prune bool) {
		if !push {
			return false
		}

		fn := n.(*ast.FuncDecl)

		if fn.Doc == nil {
			return true
		}
		obj := pass.TypesInfo.ObjectOf(fn.Name)
		if obj == nil {
			return true
		}
		doc := fn.Doc.Text()

		if matches := visibilityRegexp.FindAllStringSubmatch(doc, -1); matches != nil {
			fact := &VisibilityFact{Paths: make([]string, len(matches))}
			for i, m := range matches {
				fact.Paths[i] = m[1]
			}
			pass.ExportObjectFact(obj, fact)
		}

		return true
	})

	// Find calls that may be affected by visibility declarations.
	in.Nodes([]ast.Node{(*ast.CallExpr)(nil)}, func(n ast.Node, push bool) (prune bool) {
		if !push {
			return false
		}

		callee, ok := n.(*ast.CallExpr).Fun.(*ast.SelectorExpr)
		if !ok {
			return false
		}
		obj := pass.TypesInfo.ObjectOf(callee.Sel)
		if obj == nil {
			return false
		}
		var fact VisibilityFact
		if ok := pass.ImportObjectFact(obj, &fact); !ok {
			return false
		}
		visible := false
		for _, path := range fact.Paths {
			if path == pass.Pkg.Path() {
				visible = true
				break
			}
		}
		if !visible {
			pass.Reportf(callee.Pos(), "function %s is not visible in this package", callee.Sel.Name)
		}

		return false
	})

	return nil, nil
}
-- cgogen.go --
// cgogen reports diagnostics on files generated by cgo.
package cgogen

import (
	"go/ast"
	"strings"

	"golang.org/x/tools/go/analysis"
)

const doc = "report synthetic diagnostics on files generated by cgo"

var Analyzer = &analysis.Analyzer{
	Name: "cgogen",
	Run:  run,
	Doc:  doc,
}

func run(pass *analysis.Pass) (interface{}, error) {
	for _, f := range pass.Files {
		ast.Inspect(f, func(n ast.Node) bool {
			switch n := n.(type) {
			case *ast.Comment:
				if strings.HasPrefix(n.Text, "//go:linkname") || strings.HasPrefix(n.Text, "//go:cgo_import") {
					pass.Reportf(n.Pos(), "nogo must not run on code generated by cgo")
				}
				return true
			}
			return true
		})
	}
	return nil, nil
}

-- panicalyzer.go --
// panicalyzer panics on functions named "ShouldPanic".
package panicalyzer

import (
	"go/ast"

	"golang.org/x/tools/go/analysis"
)

const doc = "panic on functions named \"ShouldPanic\""

var Analyzer = &analysis.Analyzer{
	Name: "panicalyzer",
	Run:  run,
	Doc:  doc,
}

func run(pass *analysis.Pass) (interface{}, error) {
	for _, f := range pass.Files {
		ast.Inspect(f, func(n ast.Node) bool {
			switch n := n.(type) {
			case *ast.FuncDecl:
				if n.Name.Name == "ShouldPanic" {
					panic("function must not be named ShouldPanic")
				}
				return true
			}
			return true
		})
	}
	return nil, nil
}

-- config.json --
{
  "importfmt": {
    "only_files": {
      "has_errors\\.go": ""
    }
  },
  "foofuncname": {
    "description": "no exemptions since we know this check is 100% accurate"
  },
  "visibility": {
    "exclude_files": {
      "has_.*\\.go": "special exception to visibility rules"
    }
  }
}

-- baseconfig.json --
{
  "_base": {
    "exclude_files": {
      "has_.*\\.go": "Visibility analyzer not specified. Still inherits this special exception."
    }
  },
  "importfmt": {
    "only_files": {
      "has_errors\\.go": ""
    }
  },
  "foofuncname": {
    "description": "no exemptions since we know this check is 100% accurate, so override base config",
    "exclude_files": {}
  },
  "panicalyzer": {
	"description": "no exemptions",
	"exclude_files": {}
  }
}

-- has_errors.go --
package haserrors

import (
	_ "fmt" // This should fail importfmt

	"dep"
)

func Foo() bool { // This should fail foofuncname
	dep.D() // This should fail visibility
	return true
}

-- has_errors_linedirective.go --
//line linedirective.go:1
package haserrors_linedirective

import (
	/*line linedirective_importfmt.go:4*/ _ "fmt" // This should fail importfmt

	"dep"
)

//line linedirective_foofuncname.go:9
func Foo() bool { // This should fail foofuncname
//line linedirective_visibility.go:10
	dep.D() // This should fail visibility
	return true
}

-- panics.go --
package panics

func ShouldPanic() {
}

-- no_errors.go --
// package noerrors contains no analyzer errors.
package noerrors

import "dep"

func Baz() int {
	dep.D()
	return 1
}

-- dep.go --
package dep

// visibility:noerrors
func D() {
}

-- examplepkg/uses_cgo_clean.go --
package examplepkg

// #include <stdlib.h>
import "C"

func Bar() bool {
  if C.rand() > 10 {
    return true
  }
  return false
}

-- examplepkg/pure_src_with_err_calling_native.go --
package examplepkg

func Foo() bool { // This should fail foofuncname
  return Bar()
}

-- type_check_fail.go --
package type_check_fail

import (
	"strings"
)

func Foo() bool {
  return strings.Split("a,b,c", ",") // This fails type-checking
}
`,
	})
}

func Test(t *testing.T) {
	for _, test := range []struct {
		desc, config, target string
		wantSuccess          bool
		includes, excludes   []string
		bazelArgs            []string
	}{
		{
			desc:        "default_config",
			target:      "//:has_errors",
			wantSuccess: false,
			includes: []string{
				`has_errors.go:.*package fmt must not be imported \(importfmt\)`,
				`has_errors.go:.*function must not be named Foo \(foofuncname\)`,
				`has_errors.go:.*function D is not visible in this package \(visibility\)`,
			},
		}, {
			desc:        "default_config_linedirective",
			target:      "//:has_errors_linedirective",
			wantSuccess: false,
			includes: []string{
				`linedirective_importfmt.go:.*package fmt must not be imported \(importfmt\)`,
				`linedirective_foofuncname.go:.*function must not be named Foo \(foofuncname\)`,
				`linedirective_visibility.go:.*function D is not visible in this package \(visibility\)`,
			},
		}, {
			desc:        "custom_config",
			config:      "config.json",
			target:      "//:has_errors",
			wantSuccess: false,
			includes: []string{
				`has_errors.go:.*package fmt must not be imported \(importfmt\)`,
				`has_errors.go:.*function must not be named Foo \(foofuncname\)`,
			},
			excludes: []string{
				`visib`,
			},
		}, {
			desc:        "custom_config_linedirective",
			config:      "config.json",
			target:      "//:has_errors_linedirective",
			wantSuccess: false,
			includes: []string{
				`linedirective_foofuncname.go:.*function must not be named Foo \(foofuncname\)`,
				`linedirective_visibility.go:.*function D is not visible in this package \(visibility\)`,
			},
			excludes: []string{
				`importfmt`,
			},
		}, {
			desc:        "custom_config_with_base_linedirective",
			config:      "baseconfig.json",
			target:      "//:has_errors_linedirective",
			wantSuccess: false,
			includes: []string{
				`linedirective_foofuncname.go:.*function must not be named Foo \(foofuncname\)`,
				`linedirective_visibility.go:.*function D is not visible in this package \(visibility\)`,
			},
			excludes: []string{
				`importfmt`,
			},
		}, {
			desc:        "uses_cgo_with_errors",
			config:      "config.json",
			target:      "//:uses_cgo_with_errors",
			wantSuccess: false,
			includes: []string{
				// note the cross platform regex :)
				`examplepkg[\\/]pure_src_with_err_calling_native.go:.*function must not be named Foo \(foofuncname\)`,
			},
		}, {
			desc:        "type_check_fail",
			config:      "config.json",
			target:      "//:type_check_fail",
			wantSuccess: false,
			includes: []string{
				"\\d analyzers skipped due to type-checking error: type_check_fail.go:8:10:",
			},
			// Ensure that nogo runs even though compilation fails
			bazelArgs: []string{"--keep_going"},
		}, {
			desc:        "no_errors",
			target:      "//:no_errors",
			wantSuccess: true,
			excludes:    []string{"no_errors.go"},
		}, {
			desc:        "uses_cgo_clean",
			target:      "//:uses_cgo_clean",
			wantSuccess: true,
		}, {
			desc:        "panics",
			target:      "//:panics",
			wantSuccess: false,
			includes: []string{
				"panic: function must not be named ShouldPanic",
			},
		},
	} {
		t.Run(test.desc, func(t *testing.T) {
			if test.config != "" {
				customConfig := fmt.Sprintf("config = %q,", test.config)
				if err := replaceInFile("BUILD.bazel", origConfig, customConfig); err != nil {
					t.Fatal(err)
				}
				defer replaceInFile("BUILD.bazel", customConfig, origConfig)
			}

			cmd := bazel_testing.BazelCmd(append([]string{"build", test.target}, test.bazelArgs...)...)
			stderr := &bytes.Buffer{}
			cmd.Stderr = stderr
			if err := cmd.Run(); err == nil && !test.wantSuccess {
				t.Fatalf("unexpected success\n%s", stderr)
			} else if err != nil && test.wantSuccess {
				t.Fatalf("unexpected error: %v\n%s", err, stderr)
			}

			for _, pattern := range test.includes {
				if matched, err := regexp.Match(pattern, stderr.Bytes()); err != nil {
					t.Fatal(err)
				} else if !matched {
					t.Errorf("got output:\n %s\n which does not contain pattern: %s", string(stderr.Bytes()), pattern)
				}
			}
			for _, pattern := range test.excludes {
				if matched, err := regexp.Match(pattern, stderr.Bytes()); err != nil {
					t.Fatal(err)
				} else if matched {
					t.Errorf("output contained pattern: %s", pattern)
				}
			}
		})
	}
}

func replaceInFile(path, old, new string) error {
	data, err := ioutil.ReadFile(path)
	if err != nil {
		return err
	}
	data = bytes.ReplaceAll(data, []byte(old), []byte(new))
	return ioutil.WriteFile(path, data, 0666)
}
