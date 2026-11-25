// Copyright 2023 The Bazel Authors. All rights reserved.
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

package go_bin_runner_test

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
load("@rules_shell//shell:sh_binary.bzl", "sh_binary")

sh_binary(
    name = "go_version",
    srcs = ["go_version.sh"],
    env = {"GO": "$(rlocationpath @io_bazel_rules_go//go)"},
    data = ["@io_bazel_rules_go//go"],
    deps = ["@bazel_tools//tools/bash/runfiles"],
)

genrule(
    name = "foo",
    outs = ["bar"],
    tools = ["@io_bazel_rules_go//go"],
    cmd = "$(location @io_bazel_rules_go//go) > $@",
)

-- go_version.sh --
# --- begin runfiles.bash initialization v2 ---
# Copy-pasted from the Bazel Bash runfiles library v2.
set -uo pipefail; f=bazel_tools/tools/bash/runfiles/runfiles.bash
source "${RUNFILES_DIR:-/dev/null}/$f" 2>/dev/null || \
  source "$(grep -sm1 "^$f " "${RUNFILES_MANIFEST_FILE:-/dev/null}" | cut -f2- -d' ')" 2>/dev/null || \
  source "$0.runfiles/$f" 2>/dev/null || \
  source "$(grep -sm1 "^$f " "$0.runfiles_manifest" | cut -f2- -d' ')" 2>/dev/null || \
  source "$(grep -sm1 "^$f " "$0.exe.runfiles_manifest" | cut -f2- -d' ')" 2>/dev/null || \
  { echo>&2 "ERROR: cannot find $f"; exit 1; }; f=; set -e
# --- end runfiles.bash initialization v2 ---

$(rlocation "$GO") version
`,
		ModuleFileSuffix: `
bazel_dep(name = "rules_shell", version = "0.3.0")
`,
	})
}

func TestGoEnv(t *testing.T) {
	// Set an invalid GOROOT to test that the //go target still finds the expected hermetic GOROOT.
	os.Setenv("GOROOT", "invalid")

	bazelInfoOut, err := bazel_testing.BazelOutput("info", "output_base")
	if err != nil {
		t.Fatal(err)
	}
	outputBase := strings.TrimSpace(string(bazelInfoOut))

	goEnvOut, err := bazel_testing.BazelOutput("run", "@io_bazel_rules_go//go", "--", "env", "GOROOT")
	if err != nil {
		t.Fatal(err)
	}

	// The SDK's canonical repository name isn't known statically, so only
	// check that GOROOT points into the output base's external directory.
	goRoot := strings.TrimSpace(string(goEnvOut))
	externalDir := filepath.Join(outputBase, "external")
	if filepath.Dir(goRoot) != externalDir || !strings.Contains(filepath.Base(goRoot), "go_sdk") {
		t.Fatalf("GOROOT %s was not a Go SDK below %s", goRoot, externalDir)
	}
}

func TestGoVersionFromScript(t *testing.T) {
	err := os.Chmod("go_version.sh", 0755)
	if err != nil {
		t.Fatal(err)
	}

	goVersionOut, err := bazel_testing.BazelOutput("run", "//:go_version")
	if err != nil {
		t.Fatal(err)
	}

	if !strings.HasPrefix(string(goVersionOut), "go version go1.") {
		t.Fatalf("go version output did not start with \"go version go1.\": %s", string(goVersionOut))
	}
}

func TestNoGoInExec(t *testing.T) {
	_, err := bazel_testing.BazelOutput("build", "//:foo")
	if err == nil {
		t.Fatal("expected build to fail")
	}
	stderr := string(err.(*bazel_testing.StderrExitError).Err.Stderr)
	if !strings.Contains(stderr, "//go is only meant to be used with 'bazel run'") {
		t.Fatalf("expected \"//go is only meant to be used with 'bazel run'\" in stderr, got %s", stderr)
	}
}
