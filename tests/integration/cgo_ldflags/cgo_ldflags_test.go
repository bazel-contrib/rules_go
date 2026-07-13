// Copyright 2026 The Bazel Authors. All rights reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package cgo_ldflags_test

import (
	"testing"

	"github.com/bazelbuild/rules_go/go/tools/bazel_testing"
)

func TestMain(m *testing.M) {
	bazel_testing.TestMain(m, bazel_testing.Args{
		Main: `
-- BUILD.bazel --
load("@io_bazel_rules_go//go:def.bzl", "go_binary")
load("@rules_cc//cc:cc_library.bzl", "cc_library")

cc_library(
    name = "answer",
    srcs = ["answer.c"],
    hdrs = ["answer.h"],
)

go_binary(
    name = "cgo_ldflags",
    srcs = ["main.go"],
    cdeps = [":answer"],
    cgo = True,
    # Go 1.27 expands response files before parsing cgo flags. Keep multiple
    # linker flags here so an unquoted response file fails the build.
    clinkopts = [
        "-g",
        "-v",
    ],
    pure = "off",
)

-- answer.h --
int answer(void);

-- answer.c --
#include "answer.h"

int answer(void) {
    return 42;
}

-- main.go --
package main

/*
#include "answer.h"
*/
import "C"

import (
	"fmt"
	"runtime"
)

func main() {
	if got := runtime.Version(); got != "go1.27rc1" {
		panic(fmt.Sprintf("built with %s, want go1.27rc1", got))
	}
	if got := int(C.answer()); got != 42 {
		panic(fmt.Sprintf("C answer = %d, want 42", got))
	}
}
`,
		ModuleFileSuffix: `
bazel_dep(name = "rules_cc", version = "0.1.5")

cc_configure = use_extension("@rules_cc//cc:extensions.bzl", "cc_configure_extension")
use_repo(cc_configure, "local_config_cc")

go_sdk = use_extension("@io_bazel_rules_go//go:extensions.bzl", "go_sdk")
go_sdk.download(
    name = "go_1_27_rc1",
    sdks = {
        "darwin_amd64": ["go1.27rc1.darwin-amd64.tar.gz", "ec2abfa675a1882a13fd72035bdc50635b5b4a2b424c1f692a5737b0a333a322"],
        "darwin_arm64": ["go1.27rc1.darwin-arm64.tar.gz", "7b7b4d66fc0a7bc8e57b3602340dfef46d4f5f95fc5d30e5c5af6a671830de54"],
        "linux_amd64": ["go1.27rc1.linux-amd64.tar.gz", "102a6055d682b1f233bc1741122cc6fddae7a7dded1305fbcc30079984187144"],
        "linux_arm64": ["go1.27rc1.linux-arm64.tar.gz", "e9338b657430c7c32fffec696ce7d7b0286f99b65f8f90a2f5a6781a34f34928"],
        "windows_amd64": ["go1.27rc1.windows-amd64.zip", "10d2c755c76ca94008558bb9ec93154529ea1c0a38abe938d9b6406a9d18ffe3"],
    },
    version = "1.27rc1",
)
`,
	})
}

func TestGo127CgoWithMultipleLinkerFlags(t *testing.T) {
	if err := bazel_testing.RunBazel(
		"run",
		"--@io_bazel_rules_go//go/toolchain:sdk_version=1.27rc1",
		"//:cgo_ldflags",
	); err != nil {
		t.Fatal(err)
	}
}
