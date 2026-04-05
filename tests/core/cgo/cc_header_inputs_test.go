// Copyright 2025 The Bazel Authors. All rights reserved.
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

package cc_header_inputs_test

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

# Wrapper cc_library that transitively depends on the cc_import in the external repo.
cc_library(
    name = "greeting_wrapper",
    deps = ["@greeting_repo//:greeting"],
)

go_binary(
    name = "use_greeting",
    srcs = ["use_greeting.go"],
    cdeps = [":greeting_wrapper"],
    cgo = True,
    target_compatible_with = ["@platforms//os:linux"],
)
-- use_greeting.go --
package main

// #include <greeting.h>
import "C"

func main() {
	if C.greeting() != 42 {
		panic("unexpected value")
	}
}
-- greeting_repo/WORKSPACE --
-- greeting_repo/BUILD.bazel --
load("@rules_cc//cc:cc_import.bzl", "cc_import")
load("@rules_cc//cc:cc_binary.bzl", "cc_binary")

cc_binary(
    name = "libgreeting.so",
    srcs = ["greeting.c"],
    linkshared = True,
)

cc_import(
    name = "greeting",
    hdrs = ["include/greeting.h"],
    shared_library = ":libgreeting.so",
    includes = ["include"],
    visibility = ["//visibility:public"],
)
-- greeting_repo/greeting.c --
int greeting(void) { return 42; }
-- greeting_repo/include/greeting.h --
#ifndef GREETING_H
#define GREETING_H
int greeting(void);
#endif
`,
		WorkspaceSuffix: `
local_repository(
    name = "greeting_repo",
    path = "greeting_repo",
)
`,
	})
}

// TestTransitiveCcHeaders verifies that headers provided by a cc_import in an
// external repository are available during CGo compilation when accessed transitively through a
// wrapper cc_library.
func TestTransitiveCcHeaders(t *testing.T) {
	if out, err := bazel_testing.BazelOutput("build", "//:use_greeting"); err != nil {
		t.Fatalf("bazel build //:use_greeting failed: %v\n%s", err, out)
	}
}

// TestTransitiveCcHeadersExternalIncludePaths verifies the same as above, but
// with the external_include_paths CC toolchain feature enabled.
// When this feature is active, cc_common.compile() stores include paths for external
// repos in compilation_context.external_includes instead of includes or
// system_includes.
func TestTransitiveCcHeadersExternalIncludePaths(t *testing.T) {
	if out, err := bazel_testing.BazelOutput("build", "--features=external_include_paths", "//:use_greeting"); err != nil {
		t.Fatalf("bazel build //:use_greeting failed: %v\n%s", err, out)
	}
}
