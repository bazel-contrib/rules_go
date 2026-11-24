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

package lazy_fetch_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/bazelbuild/rules_go/go/tools/bazel_testing"
)

func TestMain(m *testing.M) {
	bazel_testing.TestMain(m, bazel_testing.Args{
		Main: `
-- BUILD.bazel --
load(":defs.bzl", "dummy_toolchain", "needs_toolchain")

toolchain_type(name = "toolchain_type")

dummy_toolchain(name = "dummy")

toolchain(
    name = "dummy_toolchain",
    toolchain = ":dummy",
    toolchain_type = ":toolchain_type",
)

needs_toolchain(name = "target")
-- defs.bzl --
def _dummy_toolchain_impl(ctx):
    return [platform_common.ToolchainInfo()]

dummy_toolchain = rule(implementation = _dummy_toolchain_impl)

def _needs_toolchain_impl(ctx):
    out = ctx.actions.declare_file(ctx.label.name + ".txt")
    ctx.actions.write(out, "")
    return [DefaultInfo(files = depset([out]))]

needs_toolchain = rule(
    implementation = _needs_toolchain_impl,
    toolchains = ["//:toolchain_type"],
)
`,
		ModuleFileSuffix: `
register_toolchains("//:dummy_toolchain")
`,
	})
}

func TestNogoNotFetched(t *testing.T) {
	// go_bazel_test shares one output base between tests, so drop any
	// repositories that earlier tests with Go targets fetched.
	if err := bazel_testing.RunBazel("clean", "--expunge"); err != nil {
		t.Fatal(err)
	}
	if err := bazel_testing.RunBazel("build", "//:target"); err != nil {
		t.Fatal(err)
	}
	out, err := bazel_testing.BazelOutput("info", "output_base")
	if err != nil {
		t.Fatal(err)
	}
	external := filepath.Join(strings.TrimSpace(string(out)), "external")
	entries, err := os.ReadDir(external)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if strings.Contains(e.Name(), "io_bazel_rules_nogo") {
			t.Errorf("building a non-Go target fetched %s", e.Name())
		}
	}
}
