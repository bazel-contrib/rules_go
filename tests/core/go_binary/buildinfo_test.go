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

package buildinfo_test

import (
	"encoding/json"
	"testing"

	"github.com/bazelbuild/rules_go/go/tools/bazel_testing"
)

func TestMain(m *testing.M) {
	bazel_testing.TestMain(m, bazel_testing.Args{
		Main: `
-- BUILD.bazel --
load("@io_bazel_rules_go//go:def.bzl", "go_binary")

go_binary(
    name = "with_dep",
    srcs = ["with_dep.go"],
    deps = ["@com_github_google_go_cmp//cmp:go_default_library"],
)

go_binary(
    name = "stdlib_only",
    srcs = ["stdlib_only.go"],
)

go_binary(
    name = "with_versionless_dep",
    srcs = ["with_versionless_dep.go"],
    deps = ["@com_example_versionless//:go_default_library"],
)
-- with_dep.go --
package main

import (
    "encoding/json"
    "os"
    "runtime/debug"

    "github.com/google/go-cmp/cmp"
)

type dep struct {
    Path    string ` + "`json:\"path\"`" + `
    Version string ` + "`json:\"version\"`" + `
}

type output struct {
    OK          bool   ` + "`json:\"ok\"`" + `
    MainPath    string ` + "`json:\"main_path\"`" + `
    MainVersion string ` + "`json:\"main_version\"`" + `
    Deps        []dep  ` + "`json:\"deps\"`" + `
}

func main() {
    _ = cmp.Equal("same", "same")

    info, ok := debug.ReadBuildInfo()
    out := output{OK: ok}
    if info != nil {
        out.MainPath = info.Main.Path
        out.MainVersion = info.Main.Version
        for _, module := range info.Deps {
            out.Deps = append(out.Deps, dep{Path: module.Path, Version: module.Version})
        }
    }
    _ = json.NewEncoder(os.Stdout).Encode(out)
}

-- stdlib_only.go --
package main

import (
    "encoding/json"
    "os"
    "runtime/debug"
)

type output struct {
    OK          bool   ` + "`json:\"ok\"`" + `
    MainPath    string ` + "`json:\"main_path\"`" + `
    MainVersion string ` + "`json:\"main_version\"`" + `
    DepCount    int    ` + "`json:\"dep_count\"`" + `
}

func main() {
    info, ok := debug.ReadBuildInfo()
    out := output{OK: ok}
    if info != nil {
        out.MainPath = info.Main.Path
        out.MainVersion = info.Main.Version
        out.DepCount = len(info.Deps)
    }
    _ = json.NewEncoder(os.Stdout).Encode(out)
}

-- with_versionless_dep.go --
package main

import (
    "encoding/json"
    "os"
    "runtime/debug"

    versionless "example.com/versionless"
)

type dep struct {
    Path    string ` + "`json:\"path\"`" + `
    Version string ` + "`json:\"version\"`" + `
}

type output struct {
    OK          bool   ` + "`json:\"ok\"`" + `
    MainPath    string ` + "`json:\"main_path\"`" + `
    MainVersion string ` + "`json:\"main_version\"`" + `
    Deps        []dep  ` + "`json:\"deps\"`" + `
}

func main() {
    _ = versionless.Name()

    info, ok := debug.ReadBuildInfo()
    out := output{OK: ok}
    if info != nil {
        out.MainPath = info.Main.Path
        out.MainVersion = info.Main.Version
        for _, module := range info.Deps {
            out.Deps = append(out.Deps, dep{Path: module.Path, Version: module.Version})
        }
    }
    _ = json.NewEncoder(os.Stdout).Encode(out)
}

-- deps/com_github_google_go_cmp/MODULE.bazel --
module(name = "com_github_google_go_cmp")

bazel_dep(name = "rules_go", repo_name = "io_bazel_rules_go")
bazel_dep(name = "rules_license", version = "1.0.0")

-- deps/com_github_google_go_cmp/BUILD.bazel --
load("@rules_license//rules:package_info.bzl", "package_info")

package_info(
    name = "gazelle_generated_package_info",
    package_name = "github.com/google/go-cmp",
    package_version = "0.6.0",
    visibility = ["//:__subpackages__"],
)

-- deps/com_github_google_go_cmp/cmp/BUILD.bazel --
load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "cmp",
    srcs = ["cmp.go"],
    importpath = "github.com/google/go-cmp/cmp",
    applicable_licenses = ["//:gazelle_generated_package_info"],
    visibility = ["//visibility:public"],
)

alias(
    name = "go_default_library",
    actual = ":cmp",
    visibility = ["//visibility:public"],
)

-- deps/com_github_google_go_cmp/cmp/cmp.go --
package cmp

func Equal(x, y string) bool {
    return x == y
}

-- deps/com_example_versionless/MODULE.bazel --
module(name = "com_example_versionless")

bazel_dep(name = "rules_go", repo_name = "io_bazel_rules_go")
bazel_dep(name = "rules_license", version = "1.0.0")

-- deps/com_example_versionless/BUILD.bazel --
load("@io_bazel_rules_go//go:def.bzl", "go_library")
load("@rules_license//rules:package_info.bzl", "package_info")

package_info(
    name = "gazelle_generated_package_info",
    package_name = "example.com/versionless",
    package_version = "",
    visibility = ["//:__subpackages__"],
)

go_library(
    name = "versionless",
    srcs = ["versionless.go"],
    importpath = "example.com/versionless",
    applicable_licenses = [":gazelle_generated_package_info"],
    visibility = ["//visibility:public"],
)

alias(
    name = "go_default_library",
    actual = ":versionless",
    visibility = ["//visibility:public"],
)

-- deps/com_example_versionless/versionless.go --
package versionless

func Name() string {
    return "versionless"
}
`,
		ModuleFileSuffix: `
bazel_dep(name = "rules_license", version = "1.0.0")
bazel_dep(name = "com_github_google_go_cmp")
bazel_dep(name = "com_example_versionless")

local_path_override(
    module_name = "com_github_google_go_cmp",
    path = "deps/com_github_google_go_cmp",
)

local_path_override(
    module_name = "com_example_versionless",
    path = "deps/com_example_versionless",
)
`,
	})
}

type dep struct {
	Path    string `json:"path"`
	Version string `json:"version"`
}

type withDepOutput struct {
	OK          bool   `json:"ok"`
	MainPath    string `json:"main_path"`
	MainVersion string `json:"main_version"`
	Deps        []dep  `json:"deps"`
}

type stdlibOnlyOutput struct {
	OK          bool   `json:"ok"`
	MainPath    string `json:"main_path"`
	MainVersion string `json:"main_version"`
	DepCount    int    `json:"dep_count"`
}

func TestReadBuildInfoDeps(t *testing.T) {
	stdout, err := bazel_testing.BazelOutput("run", "//:with_dep")
	if err != nil {
		t.Fatal(err)
	}

	var got withDepOutput
	if err := json.Unmarshal(stdout, &got); err != nil {
		t.Fatalf("unmarshal output %q: %v", stdout, err)
	}
	if !got.OK {
		t.Fatalf("ReadBuildInfo returned ok=false: %+v", got)
	}
	if got.MainPath != "" || got.MainVersion != "" {
		t.Fatalf("got Main %q %q; want empty", got.MainPath, got.MainVersion)
	}
	if len(got.Deps) == 0 {
		t.Fatalf("got no deps: %+v", got)
	}

	foundCmp := false
	for _, dep := range got.Deps {
		if dep.Path == "github.com/google/go-cmp" && dep.Version == "v0.6.0" {
			foundCmp = true
			break
		}
	}
	if !foundCmp {
		t.Fatalf("missing github.com/google/go-cmp@v0.6.0 in %+v", got.Deps)
	}
}

func TestReadBuildInfoWithoutMetadata(t *testing.T) {
	stdout, err := bazel_testing.BazelOutput("run", "//:stdlib_only")
	if err != nil {
		t.Fatal(err)
	}

	var got stdlibOnlyOutput
	if err := json.Unmarshal(stdout, &got); err != nil {
		t.Fatalf("unmarshal output %q: %v", stdout, err)
	}
	if !got.OK {
		t.Fatalf("ReadBuildInfo returned ok=false: %+v", got)
	}
	if got.MainPath != "" || got.MainVersion != "" {
		t.Fatalf("got Main %q %q; want empty", got.MainPath, got.MainVersion)
	}
	if got.DepCount != 0 {
		t.Fatalf("got %d deps; want 0", got.DepCount)
	}
}

func TestReadBuildInfoVersionlessDep(t *testing.T) {
	stdout, err := bazel_testing.BazelOutput("run", "//:with_versionless_dep")
	if err != nil {
		t.Fatal(err)
	}

	var got withDepOutput
	if err := json.Unmarshal(stdout, &got); err != nil {
		t.Fatalf("unmarshal output %q: %v", stdout, err)
	}
	if !got.OK {
		t.Fatalf("ReadBuildInfo returned ok=false: %+v", got)
	}

	foundVersionless := false
	for _, dep := range got.Deps {
		if dep.Path == "example.com/versionless" && dep.Version == "(devel)" {
			foundVersionless = true
			break
		}
	}
	if !foundVersionless {
		t.Fatalf("missing example.com/versionless@(devel) in %+v", got.Deps)
	}
}
