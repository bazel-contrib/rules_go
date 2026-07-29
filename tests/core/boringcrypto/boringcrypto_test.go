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

package boringcrypto_test

import (
	"bytes"
	"os"
	"os/exec"
	"strings"
	"testing"

	"github.com/bazelbuild/rules_go/go/tools/bazel_testing"
)

func TestMain(m *testing.M) {
	bazel_testing.TestMain(m, bazel_testing.Args{
		Main: `
-- BUILD.bazel --
load("@io_bazel_rules_go//go:def.bzl", "go_binary", "go_library")

go_binary(
    name = "program",
    srcs = ["main.go"],
	deps = [":library"],
	visibility = ["//visibility:public"],
)

go_library(
	name = "library",
	srcs = ["library.go"],
	importpath = "example.com/library"
)
-- main.go --
//go:build goexperiment.boringcrypto

package main

import "example.com/library"

func main() {
	library.F()
}
-- library.go --
package library

func F() {}
`,
	})
}

const origWrapSDK = `_host_go_sdk.wrap(
    root_file = "@local_go_sdk//:ROOT",
)`

const wrapSDKBoringcrypto = `_host_go_sdk.wrap(
    root_file = "@local_go_sdk//:ROOT",
    experiments = ["boringcrypto"],
)`

func TestBoringcryptoExperimentPresent(t *testing.T) {
	mustReplaceInFile(t, "MODULE.bazel", origWrapSDK, wrapSDKBoringcrypto)
	defer mustReplaceInFile(t, "MODULE.bazel", wrapSDKBoringcrypto, origWrapSDK)

	if _, err := exec.LookPath("go"); err != nil {
		t.Skip("go command is necessary to evaluate if boringcrypto experiment is present")
	}

	cmd := bazel_testing.BazelCmd("build", "//:program")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stdout
	if err := cmd.Run(); err != nil {
		t.Fatal("failed to run bazel build: ", err)
	}

	out, err := exec.Command("go", "version", "bazel-bin/program_/program").CombinedOutput()
	if err != nil {
		t.Fatalf("failed to run go version command: %v\noutput was:\n%v", err, string(out))
	}

	if !strings.Contains(string(out), "X:boringcrypto") {
		t.Fatalf(`version of binary: got %q, want string containing "X:boringcrypto"`, string(out))
	}
}

func mustReplaceInFile(t *testing.T, path, old, new string) {
	t.Helper()
	if old == new {
		return
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(data, []byte(old)) {
		t.Fatalf("bytes to replace %q not found in file %q with contents, %q", old, path, data)
	}
	data = bytes.ReplaceAll(data, []byte(old), []byte(new))
	if err := os.WriteFile(path, data, 0666); err != nil {
		t.Fatal(err)
	}
}
