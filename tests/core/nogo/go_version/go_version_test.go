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

package goversion_test

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
load("@io_bazel_rules_go//go:def.bzl", "go_binary", "go_library", "nogo")

nogo(
    name = "nogo",
    visibility = ["//visibility:public"],
    deps = [":goversion"],
)

go_library(
    name = "goversion",
    srcs = [
        "goversion.go",
        "goversion_go121.go",
        "goversion_go122.go",
        "goversion_pre121.go",
        "goversion_pre122.go",
        "goversion_runtime.go",
    ],
    importpath = "goversionanalyzer",
    deps = ["@org_golang_x_tools//go/analysis"],
    visibility = ["//visibility:public"],
)

go_library(
    name = "src_default",
    srcs = ["src_default.go"],
    importpath = "srcdefault",
)

go_binary(
    name = "sdk_version",
    srcs = ["sdk_version.go"],
)

-- goversion.go --
package goversion

import "golang.org/x/tools/go/analysis"

var Analyzer = &analysis.Analyzer{
	Name: "goversion",
	Doc:  "checks that nogo plumbs the SDK Go version into go/types",
	Run:  run,
}

func run(pass *analysis.Pass) (interface{}, error) {
	want := wantGoVersion()
	checkPackageGoVersion(pass, want)
	checkFileVersions(pass, want)
	return nil, nil
}

-- goversion_runtime.go --
package goversion

import (
	"runtime"
	"strings"
)

func wantGoVersion() string {
	version := runtime.Version()
	if strings.HasPrefix(version, "devel ") {
		for _, field := range strings.Fields(version) {
			if strings.HasPrefix(field, "go") {
				return field
			}
		}
	}
	return version
}

-- goversion_go121.go --
//go:build go1.21
// +build go1.21

package goversion

import "golang.org/x/tools/go/analysis"

func checkPackageGoVersion(pass *analysis.Pass, want string) {
	if got := pass.Pkg.GoVersion(); got != want {
		pass.Reportf(pass.Files[0].Package, "package GoVersion = %q, want %q", got, want)
	}
}

-- goversion_pre121.go --
//go:build !go1.21
// +build !go1.21

package goversion

import "golang.org/x/tools/go/analysis"

func checkPackageGoVersion(*analysis.Pass, string) {}

-- goversion_go122.go --
//go:build go1.22
// +build go1.22

package goversion

import "golang.org/x/tools/go/analysis"

func checkFileVersions(pass *analysis.Pass, want string) {
	if pass.TypesInfo.FileVersions == nil {
		pass.Reportf(pass.Files[0].Package, "missing FileVersions map")
		return
	}
	for _, file := range pass.Files {
		v, ok := pass.TypesInfo.FileVersions[file]
		if !ok {
			pass.Reportf(file.Package, "missing FileVersions entry")
			continue
		}
		if v != want {
			pass.Reportf(file.Package, "file version = %q, want %q", v, want)
		}
	}
}

-- goversion_pre122.go --
//go:build !go1.22
// +build !go1.22

package goversion

import "golang.org/x/tools/go/analysis"

func checkFileVersions(*analysis.Pass, string) {}

-- sdk_version.go --
package main

import (
	"fmt"
	"runtime"
	"strings"
)

func main() {
	version := runtime.Version()
	if strings.HasPrefix(version, "devel ") {
		for _, field := range strings.Fields(version) {
			if strings.HasPrefix(field, "go") {
				fmt.Print(strings.TrimPrefix(field, "go"))
				return
			}
		}
	}
	fmt.Print(strings.TrimPrefix(version, "go"))
}

-- src_default.go --
package srcdefault

func Default() {}
`,
	})
}

func TestGoVersion(t *testing.T) {
	if err := bazel_testing.RunBazel("build", "//:src_default"); err != nil {
		t.Fatal(err)
	}
}

func TestRunNogoActionUsesSdkGoVersion(t *testing.T) {
	versionOut, err := bazel_testing.BazelOutput("run", "//:sdk_version")
	if err != nil {
		t.Fatal(err)
	}

	want := strings.TrimSpace(string(versionOut))
	out, err := bazel_testing.BazelOutput(
		"aquery",
		"--include_commandline",
		"--include_param_files",
		`mnemonic("RunNogo", //:src_default)`,
	)
	if err != nil {
		t.Fatal(err)
	}

	action := string(out)
	if !strings.Contains(action, "-go_version") || !strings.Contains(action, want) {
		t.Fatalf("RunNogo action missing raw SDK go_version %q:\n%s", want, action)
	}
	if strings.Contains(action, "-gcflags") {
		t.Fatalf("RunNogo action should not depend on gcflags:\n%s", action)
	}
}
