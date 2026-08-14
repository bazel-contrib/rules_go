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
load("@io_bazel_rules_go//go:def.bzl", "go_binary", "go_test")
load("@package_metadata//rules:package_metadata.bzl", "package_metadata")

package_metadata(
    name = "main_package_metadata",
    purl = "pkg:golang/example.com/main@v1.0.0",
)

go_binary(
    name = "with_dep",
    srcs = ["with_dep.go"],
    applicable_licenses = [":main_package_metadata"],
    deps = [
        "@com_example_versionless//:go_default_library",  # Declared but not imported.
        "@com_github_google_go_cmp//cmp:go_default_library",
    ],
)

go_test(
    name = "with_dep_test",
    srcs = ["with_dep_test.go"],
    applicable_licenses = [":main_package_metadata"],
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

go_binary(
    name = "with_vendored_dep",
    srcs = ["with_vendored_dep.go"],
    deps = ["//third_party/vendored:vendored"],
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

-- with_dep_test.go --
package buildinfo

import (
    "runtime/debug"
    "testing"

    "github.com/google/go-cmp/cmp"
)

func TestBuildInfoDeps(t *testing.T) {
    _ = cmp.Equal("same", "same")

    info, ok := debug.ReadBuildInfo()
    if !ok {
        t.Fatal("ReadBuildInfo returned ok=false")
    }

    foundCmp := false
    for _, dep := range info.Deps {
        if dep.Path == "example.com/main" {
            t.Fatalf("test target metadata was included as a dependency: %+v", info.Deps)
        }
        if dep.Path == "github.com/google/go-cmp" && dep.Version == "v0.6.0" {
            foundCmp = true
        }
    }
    if !foundCmp {
        t.Fatalf("missing github.com/google/go-cmp@v0.6.0 in %+v", info.Deps)
    }
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

-- with_vendored_dep.go --
package main

import (
    "encoding/json"
    "os"
    "runtime/debug"

    "example.com/vendored"
)

type dep struct {
    Path    string ` + "`json:\"path\"`" + `
    Version string ` + "`json:\"version\"`" + `
}

type output struct {
    OK   bool  ` + "`json:\"ok\"`" + `
    Deps []dep ` + "`json:\"deps\"`" + `
}

func main() {
    _ = vendored.Name()

    info, ok := debug.ReadBuildInfo()
    out := output{OK: ok}
    if info != nil {
        for _, module := range info.Deps {
            out.Deps = append(out.Deps, dep{Path: module.Path, Version: module.Version})
        }
    }
    _ = json.NewEncoder(os.Stdout).Encode(out)
}

-- third_party/vendored/BUILD.bazel --
load("@io_bazel_rules_go//go:def.bzl", "go_library")
load("@package_metadata//rules:package_metadata.bzl", "package_metadata")

package_metadata(
    name = "package_metadata",
    purl = "pkg:golang/example.com/vendored@v1.2.3",
)

go_library(
    name = "vendored",
    srcs = ["vendored.go"],
    importpath = "example.com/vendored",
    applicable_licenses = [":package_metadata"],
    visibility = ["//visibility:public"],
)

-- third_party/vendored/vendored.go --
package vendored

func Name() string {
    return "vendored"
}

-- deps/com_github_google_go_cmp/MODULE.bazel --
module(name = "com_github_google_go_cmp")

bazel_dep(name = "rules_go", repo_name = "io_bazel_rules_go")
bazel_dep(name = "package_metadata", version = "0.0.5")

-- deps/com_github_google_go_cmp/BUILD.bazel --
load("@package_metadata//rules:package_metadata.bzl", "package_metadata")

package_metadata(
    name = "package_metadata",
    purl = "pkg:golang/github.com/google/go-cmp@v0.6.0",
    visibility = ["//:__subpackages__"],
)

-- deps/com_github_google_go_cmp/cmp/BUILD.bazel --
load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "cmp",
    srcs = ["cmp.go"],
    importpath = "github.com/google/go-cmp/cmp",
    applicable_licenses = ["//:package_metadata"],
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
bazel_dep(name = "package_metadata", version = "0.0.5")

-- deps/com_example_versionless/BUILD.bazel --
load("@io_bazel_rules_go//go:def.bzl", "go_library")
load("@package_metadata//rules:package_metadata.bzl", "package_metadata")

package_metadata(
    name = "package_metadata",
    purl = "pkg:golang/example.com/versionless",
    visibility = ["//:__subpackages__"],
)

go_library(
    name = "versionless",
    srcs = ["versionless.go"],
    importpath = "example.com/versionless",
    applicable_licenses = [":package_metadata"],
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
bazel_dep(name = "package_metadata", version = "0.0.5")
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
	foundUnusedDeclaredDep := false
	for _, dep := range got.Deps {
		if dep.Path == "github.com/google/go-cmp" && dep.Version == "v0.6.0" {
			foundCmp = true
		}
		if dep.Path == "example.com/versionless" && dep.Version == "(devel)" {
			foundUnusedDeclaredDep = true
		}
	}
	if !foundCmp {
		t.Fatalf("missing github.com/google/go-cmp@v0.6.0 in %+v", got.Deps)
	}
	if !foundUnusedDeclaredDep {
		t.Fatalf("missing unused declared dependency example.com/versionless@(devel) in %+v", got.Deps)
	}
	for _, dep := range got.Deps {
		if dep.Path == "example.com/main" {
			t.Fatalf("binary's own metadata was included as a dependency: %+v", got.Deps)
		}
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

func TestGoTestBuildInfoDeps(t *testing.T) {
	if _, err := bazel_testing.BazelOutput("test", "//:with_dep_test"); err != nil {
		t.Fatal(err)
	}
}

func TestGoTestBuildInfoDepsWithCoverage(t *testing.T) {
	if _, err := bazel_testing.BazelOutput("coverage", "//:with_dep_test"); err != nil {
		t.Fatal(err)
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

func TestReadBuildInfoVendoredDep(t *testing.T) {
	stdout, err := bazel_testing.BazelOutput("run", "//:with_vendored_dep")
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

	for _, dep := range got.Deps {
		if dep.Path == "example.com/vendored" && dep.Version == "v1.2.3" {
			return
		}
	}
	t.Fatalf("missing example.com/vendored@v1.2.3 in %+v", got.Deps)
}
