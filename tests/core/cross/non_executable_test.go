// Copyright 2022 The Bazel Authors. All rights reserved.
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

package non_executable_test

import (
	"regexp"
	"testing"

	"github.com/bazelbuild/rules_go/go/tools/bazel_testing"
)

var errorRegexp = regexp.MustCompile(`cannot run go_cross target "host_archive": underlying target "@{0,2}//src:archive" is not executable`);

func TestMain(m *testing.M) {
	bazel_testing.TestMain(m, bazel_testing.Args{
		Main: `
-- src/BUILD.bazel --
load("@rules_cc//cc:cc_binary.bzl", "cc_binary")
load("@io_bazel_rules_go//go:def.bzl", "go_binary", "go_cross_binary")
load(":rules.bzl", "no_runfiles_check")

go_binary(
    name = "archive",
    srcs = ["archive.go"],
    cgo = True,
    linkmode = "c-archive",
)

# We make a new platform here so that we can exercise the go_cross_binary rule.
# However, the test needs to run on all hosts, so the platform needs
# to inherit from the host platform.
platform(
  name = "host_cgo",
  parents = ["@platforms//host"],
  constraint_values = [
    "@io_bazel_rules_go//go/toolchain:cgo_on",
  ],
)

go_cross_binary(
    name = "host_archive",
    target = ":archive",
    platform = ":host_cgo",
)

cc_binary(
    name = "main",
    srcs = ["main.c"],
    deps = [":host_archive"],
)

no_runfiles_check(
    name = "no_runfiles",
    target = ":main",
)
-- src/archive.go --
package main

import "C"

func main() {}
-- src/main.c --
int main() {}
-- src/rules.bzl --
def _no_runfiles_check_impl(ctx):
    runfiles = ctx.attr.target[DefaultInfo].default_runfiles.files.to_list()
    for runfile in runfiles:
        # As of Bazel 9, cc_binary adds a stub used by bazel run.
        if runfile.short_path not in ["src/main", "src/main.exe", "src/fake_executable_for_bazel_run", "src/fake_executable_for_bazel_run.bat"]:
            fail("Unexpected runfile: %s" % runfile.short_path)

no_runfiles_check = rule(
    implementation = _no_runfiles_check_impl,
    attrs = {
        "target": attr.label(),
    }
)
`,
	})
}

func TestNonExecutableGoBinaryCantBeRun(t *testing.T) {
	if err := bazel_testing.RunBazel("build", "//src:host_archive"); err != nil {
		t.Fatal(err)
	}
	err := bazel_testing.RunBazel("run", "//src:host_archive")
	if err == nil || !errorRegexp.MatchString(err.Error()) {
		t.Errorf("Expected bazel run to fail due to //src:host_archive not being executable")
	}
}

func TestNonExecutableGoBinaryNotInRunfiles(t *testing.T) {
	if err := bazel_testing.RunBazel("build", "//src:no_runfiles"); err != nil {
		t.Fatal(err)
	}
}
