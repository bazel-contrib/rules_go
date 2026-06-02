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

package exec_config_test

import (
	"strings"
	"testing"

	"github.com/bazelbuild/rules_go/go/tools/bazel_testing"
)

func TestMain(m *testing.M) {
	bazel_testing.TestMain(m, bazel_testing.Args{
		Main: `
-- BUILD.bazel --
load("@io_bazel_rules_go//go:def.bzl", "go_library", "nogo")

nogo(
    name = "my_nogo",
    vet = True,
    visibility = ["//visibility:public"],
)

go_library(
    name = "lib",
    srcs = ["lib.go"],
    importpath = "example.com/lib",
)

-- lib.go --
package lib

func Hello() string {
	return "hello"
}
`,
		ModuleFileSuffix: `
go_sdk = use_extension("@io_bazel_rules_go//go:extensions.bzl", "go_sdk")
go_sdk.nogo(nogo = "//:my_nogo")
`,
	})
}

// TestNoCycleWithExcludedStarlarkFlags is a regression test for the nogo
// bootstrapping cycle that reappears when Starlark flags are excluded from the
// exec configuration.
//
// Every Go target depends on the nogo binary (cfg = "exec"), and the nogo
// binary is itself built from Go libraries that depend on nogo. rules_go breaks
// this cycle with the //go/private:bootstrap_nogo flag, which go_tool_transition
// sets to True when building nogo so that nogo's own dependencies resolve the
// nogo alias to a noop instead of the real binary.
//
// Under --incompatible_exclude_starlark_flags_from_exec_config, that flag would
// be reset to its default across the cfg = "exec" edge into nogo's
// dependencies, re-activating nogo on them and reintroducing the cycle. Marking
// bootstrap_nogo (and request_nogo) with scope = "universal" exempts them from
// that reset, which is what this test guards.
func TestNoCycleWithExcludedStarlarkFlags(t *testing.T) {
	const flag = "--incompatible_exclude_starlark_flags_from_exec_config"
	err := bazel_testing.RunBazel("build", "//:lib", flag)
	if err == nil {
		return // Build succeeded: no cycle.
	}

	// The flag only exists on Bazel 8+. On older versions, skip rather than
	// fail so the test stays green across the supported Bazel matrix.
	if strings.Contains(err.Error(), "Unrecognized option: "+flag) {
		t.Skipf("Bazel does not support %s; skipping", flag)
	}

	if strings.Contains(err.Error(), "cycle in dependency graph") {
		t.Fatalf("nogo bootstrapping cycle reintroduced by %s:\n%s", flag, err)
	}
	t.Fatalf("unexpected build failure: %s", err)
}
