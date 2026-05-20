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

package host_mode_test

import (
	"bytes"
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
load(":host_wrap.bzl", "host_wrap")

# Real analyzer (rather than default_nogo) is what forces Bazel to walk
# nogo's deps and exercise the recursion guard.
nogo(
    name = "nogo",
    deps = [":foofuncname"],
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
    name = "clean_lib",
    srcs = ["clean_lib.go"],
    importpath = "cleanlib",
)

go_library(
    name = "has_errors",
    srcs = ["has_errors.go"],
    importpath = "haserrors",
)

host_wrap(
    name = "wrap_clean",
    dep = [":clean_lib"],
)

host_wrap(
    name = "wrap_has_errors",
    dep = [":has_errors"],
)

-- host_wrap.bzl --
# Pulls its dep via legacy host mode (the regression axis) and forwards
# _validation, which is what makes Bazel actually execute nogo on it.
def _host_wrap_impl(ctx):
    files = depset(transitive = [d[DefaultInfo].files for d in ctx.attr.dep])
    validation = depset(transitive = [
        d[OutputGroupInfo]._validation
        for d in ctx.attr.dep
        if OutputGroupInfo in d and hasattr(d[OutputGroupInfo], "_validation")
    ])
    return [
        DefaultInfo(files = files),
        OutputGroupInfo(_validation = validation),
    ]

host_wrap = rule(
    implementation = _host_wrap_impl,
    attrs = {
        "dep": attr.label_list(cfg = "host", mandatory = True),
    },
)

-- foofuncname.go --
package foofuncname

import (
	"go/ast"

	"golang.org/x/tools/go/analysis"
)

var Analyzer = &analysis.Analyzer{
	Name: "foofuncname",
	Run:  run,
	Doc:  "report functions named \"Foo\"",
}

func run(pass *analysis.Pass) (interface{}, error) {
	for _, f := range pass.Files {
		ast.Inspect(f, func(n ast.Node) bool {
			if fn, ok := n.(*ast.FuncDecl); ok {
				if fn.Name.Name == "Foo" {
					pass.Reportf(fn.Pos(), "function must not be named Foo")
				}
			}
			return true
		})
	}
	return nil, nil
}

-- clean_lib.go --
package cleanlib

func Bar() int { return 42 }

-- has_errors.go --
package haserrors

func Foo() int { return 1 }
`,
	})
}

// TestHostModeCleanLibBuilds: clean lib via cfg = "host" must build —
// catches both recursion regressions (analysis must terminate) and
// wrong-exec-platform nogo binaries ("exec format error").
func TestHostModeCleanLibBuilds(t *testing.T) {
	cmd := bazel_testing.BazelCmd("build", "//:wrap_clean")
	stderr := &bytes.Buffer{}
	cmd.Stderr = stderr
	if err := cmd.Run(); err != nil {
		t.Fatalf("expected //:wrap_clean to build successfully, got error: %v\nstderr:\n%s", err, stderr.String())
	}
	if strings.Contains(stderr.String(), "exec format error") {
		t.Fatalf("nogo binary appears to have been built for the wrong platform (exec format error)\nstderr:\n%s", stderr.String())
	}
}

// TestHostModeNogoActuallyRuns: with _validation forwarded, nogo must
// run on the host-mode dep and flag Foo().
func TestHostModeNogoActuallyRuns(t *testing.T) {
	cmd := bazel_testing.BazelCmd("build", "//:wrap_has_errors")
	stderr := &bytes.Buffer{}
	cmd.Stderr = stderr
	err := cmd.Run()
	if err == nil {
		t.Fatalf("expected //:wrap_has_errors to fail because nogo should flag Foo(), but build succeeded\nstderr:\n%s", stderr.String())
	}
	if !strings.Contains(stderr.String(), "function must not be named Foo") {
		t.Fatalf("expected nogo diagnostic 'function must not be named Foo' in output\nstderr:\n%s", stderr.String())
	}
	if strings.Contains(stderr.String(), "exec format error") {
		t.Fatalf("nogo binary appears to have been built for the wrong platform (exec format error)\nstderr:\n%s", stderr.String())
	}
}
