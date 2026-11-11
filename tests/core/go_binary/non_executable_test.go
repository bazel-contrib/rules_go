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
	"fmt"
	"os"
	"runtime"
	"strings"
	"testing"

	"github.com/bazelbuild/rules_go/go/tools/bazel_testing"
)

func TestMain(m *testing.M) {
	bazel_testing.TestMain(m, bazel_testing.Args{
		Main: `
-- src/BUILD.bazel --
load("@rules_cc//cc:cc_binary.bzl", "cc_binary")

load("@io_bazel_rules_go//go:def.bzl", "go_binary")
load("@package_metadata//rules:package_metadata.bzl", "package_metadata")
load(":rules.bzl", "no_runfiles_check")

package_metadata(
    name = "package_metadata",
    purl = "pkg:golang/example.com/archive@v1.0.0",
)

go_binary(
    name = "archive",
    srcs = ["archive.go"],
    applicable_licenses = [":package_metadata"],
    cgo = True,
    linkmode = "c-archive",
)

cc_binary(
    name = "main",
    srcs = ["main.c"],
    deps = [":archive"],
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
        if runfile.short_path not in ["src/main", "src/main.exe"]:
            fail("Unexpected runfile: %s" % runfile.short_path)

no_runfiles_check = rule(
    implementation = _no_runfiles_check_impl,
    attrs = {
        "target": attr.label(),
    }
)
`,
		ModuleFileSuffix: `
bazel_dep(name = "package_metadata", version = "0.0.5")
`,
		SetUp: func() error {
			return writeStatusCommand()
		},
	})
}

func statusCommand() string {
	if runtime.GOOS == "windows" {
		return `.\\status.bat`
	}
	return "./status.sh"
}

func writeStatusCommand() error {
	if runtime.GOOS == "windows" {
		return os.WriteFile("status.bat", []byte(fmt.Sprintf("@echo off\r\necho STABLE_VCS git\r\necho STABLE_VCS_REVISION %s\r\n", "abc123")), 0o666)
	}
	return os.WriteFile("status.sh", []byte(fmt.Sprintf("#!/bin/sh\n\necho \"STABLE_VCS git\"\necho \"STABLE_VCS_REVISION %s\"\n", "abc123")), 0o755)
}

func TestNonExecutableGoBinaryCantBeRun(t *testing.T) {
	if err := bazel_testing.RunBazel("build", "//src:archive"); err != nil {
		t.Fatal(err)
	}
	err := bazel_testing.RunBazel("run", "//src:archive")
	if err == nil || !strings.Contains(err.Error(), "ERROR: Cannot run target //src:archive: Not executable") {
		t.Errorf("Expected bazel run to fail due to //src:archive not being executable")
	}
}

func TestNonExecutableGoBinaryNotInRunfiles(t *testing.T) {
	if err := bazel_testing.RunBazel("build", "//src:no_runfiles"); err != nil {
		t.Fatal(err)
	}
}

func TestStampedNonExecutableGoBinaryOmitsVCSStamp(t *testing.T) {
	out, err := bazel_testing.BazelOutput(
		"aquery",
		"--stamp",
		"--workspace_status_command="+statusCommand(),
		`mnemonic("GoLink", //src:archive)`,
	)
	if err != nil {
		t.Fatal(err)
	}

	action := string(out)
	if !strings.Contains(action, "Mnemonic: GoLink") {
		t.Fatalf("missing GoLink action:\n%s", action)
	}
	for _, unexpected := range []string{"go_context_data.vcsstamp", "-vcs_stamp"} {
		if strings.Contains(action, unexpected) {
			t.Fatalf("unexpected VCS stamp input %q:\n%s", unexpected, action)
		}
	}
}
