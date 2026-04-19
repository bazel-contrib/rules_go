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

package cc_linker_inputs_test

import (
	"testing"

	"github.com/bazelbuild/rules_go/go/tools/bazel_testing"
)

func TestMain(m *testing.M) {
	bazel_testing.TestMain(m, bazel_testing.Args{
		Main: `
-- mock_so_library.bzl --
def _mock_so_library_impl(ctx):
    cc_toolchain = ctx.toolchains["@bazel_tools//tools/cpp:toolchain_type"].cc
    feature_configuration = cc_common.configure_features(
        ctx = ctx,
        cc_toolchain = cc_toolchain,
    )
    libs = []
    for dyn_lib in ctx.files.dynamic_libs:
        ifso = ctx.actions.declare_file(dyn_lib.basename + ".ifso")
        ctx.actions.write(ifso, "/* empty interface library */")
        lib = cc_common.create_library_to_link(
            actions = ctx.actions,
            cc_toolchain = cc_toolchain,
            feature_configuration = feature_configuration,
            interface_library = ifso,
            dynamic_library = dyn_lib,
        )
        libs.append(lib)
    linker_input = cc_common.create_linker_input(
        owner = ctx.label,
        libraries = depset(libs),
    )
    return [CcInfo(linking_context = cc_common.create_linking_context(
        linker_inputs = depset([linker_input]),
    ))]

mock_so_library = rule(
    implementation = _mock_so_library_impl,
    attrs = {
        "dynamic_libs": attr.label_list(allow_files = True),
    },
    toolchains = ["@bazel_tools//tools/cpp:toolchain_type"],
    fragments = ["cpp"],
)
-- mock_cc_deb_library.bzl --
def _mock_cc_deb_library_impl(ctx):
    expanded_linkopts = [
        ctx.expand_make_variables("linkopts", opt, {})
        for opt in ctx.attr.linkopts
    ]
    linker_input = cc_common.create_linker_input(
        owner = ctx.label,
        user_link_flags = depset(expanded_linkopts),
        additional_inputs = depset(ctx.files.additional_linker_inputs),
    )
    own_cc_info = CcInfo(linking_context = cc_common.create_linking_context(
        linker_inputs = depset([linker_input]),
    ))
    dep_cc_infos = [dep[CcInfo] for dep in ctx.attr.deps if CcInfo in dep]
    return [cc_common.merge_cc_infos(
        direct_cc_infos = [own_cc_info],
        cc_infos = dep_cc_infos,
    )]

mock_cc_deb_library = rule(
    implementation = _mock_cc_deb_library_impl,
    attrs = {
        "deps": attr.label_list(providers = [CcInfo]),
        "linkopts": attr.string_list(),
        "additional_linker_inputs": attr.label_list(allow_files = True),
    },
)
-- BUILD.bazel --
load("@io_bazel_rules_go//go:def.bzl", "go_binary")
load("@rules_cc//cc:cc_binary.bzl", "cc_binary")
load("//:mock_so_library.bzl", "mock_so_library")
load("//:mock_cc_deb_library.bzl", "mock_cc_deb_library")

cc_binary(
    name = "libgreeting.so",
    srcs = ["greeting.c"],
    linkshared = True,
)

mock_so_library(
    name = "greeting_so_lib",
    dynamic_libs = [":libgreeting.so"],
)

mock_cc_deb_library(
    name = "greeting_deb_lib",
    deps = [":greeting_so_lib"],
    additional_linker_inputs = [":libgreeting.so"],
    linkopts = [
        "-L$(BINDIR)/.",
        "-lgreeting",
    ],
)

go_binary(
    name = "use_greeting",
    srcs = ["use_greeting.go"],
    cdeps = [":greeting_deb_lib"],
    cgo = True,
    target_compatible_with = ["@platforms//os:linux"],
)

[mock_so_library(
    name = "greeting_so_lib_%d" % i,
    dynamic_libs = [":libgreeting.so"],
) for i in range(50)]

mock_cc_deb_library(
    name = "many_greeting_libs",
    deps = ["greeting_so_lib_%d" % i for i in range(50)],
    additional_linker_inputs = [":libgreeting.so"],
    linkopts = [
        "-L$(BINDIR)/.",
        "-lgreeting",
    ],
)

go_binary(
    name = "use_many_greetings",
    srcs = ["use_greeting.go"],
    cdeps = [":many_greeting_libs"],
    cgo = True,
    target_compatible_with = ["@platforms//os:linux"],
)
-- greeting.c --
int greeting() { return 42; }
-- use_greeting.go --
package main

// extern int greeting();
import "C"

func main() { C.greeting() }
`,
	})
}

// TestInterfacePlusDynamicLibrary verifies that a .so provided via
// LibraryToLink(interface_library, dynamic_library) is available in the
// GoLink sandbox and the binary links successfully.
func TestInterfacePlusDynamicLibrary(t *testing.T) {
	if out, err := bazel_testing.BazelOutput("build", "//:use_greeting"); err != nil {
		t.Fatalf("bazel build //:use_greeting failed: %v\n%s", err, out)
	}
}

// TestManyInterfacePlusDynamicLibraries verifies that many
// LibraryToLink(interface_library, dynamic_library) pairs do not cause
// "Argument list too long" from rpath flag generation.
func TestManyInterfacePlusDynamicLibraries(t *testing.T) {
	if out, err := bazel_testing.BazelOutput("build", "//:use_many_greetings"); err != nil {
		t.Fatalf("bazel build //:use_many_greetings failed: %v\n%s", err, out)
	}
}
