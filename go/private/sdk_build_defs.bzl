# Copyright 2026 The Bazel Authors. All rights reserved.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#    http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

load("@io_bazel_rules_go//go:def.bzl", "go_sdk")
load("@io_bazel_rules_go//go/private:common.bzl", "RULES_GO_STDLIB_PREFIX")
load("@io_bazel_rules_go//go/private:go_toolchain.bzl", "declare_go_toolchains")
load("@io_bazel_rules_go//go/private/rules:binary.bzl", "go_tool_binary")
load("@io_bazel_rules_go//go/private/rules:sdk.bzl", "package_list")
load("@io_bazel_rules_go//go/private/rules:transition.bzl", "non_go_reset_target")

STDLIB_SRCS_EXCLUDE = [
    "src/**/*_test.go",
    "src/**/testdata/**",
    # Only used by tests, cgo fails with linux before 3.17
    "src/crypto/internal/sysrand/internal/seccomp/**",
    "src/encoding/json/internal/jsontest/**",
    "src/log/slog/internal/benchmarks/**",
    "src/log/slog/internal/slogtest/**",
    "src/internal/obscuretestdata/**",
    "src/internal/testpty/**",
    "src/net/internal/cgotest/**",
    "src/net/internal/socktest/**",
    "src/reflect/internal/example*/**",
    "src/runtime/internal/startlinetest/**",
]

def define_sdk_repository_targets(
        *,
        experiments,
        exec_compatible_with,
        files_srcs,
        go,
        goarch,
        goos,
        go_sdk_root_file,
        package_list_srcs,
        version):
    go_sdk(
        name = "go_sdk",
        srcs = [":srcs"],
        experiments = experiments,
        go = go,
        goarch = goarch,
        goos = goos,
        headers = [":headers"],
        libs = [":libs"],
        package_list = ":package_list",
        root_file = go_sdk_root_file,
        tools = [":tools"],
        version = version,
    )

    go_tool_binary(
        name = "builder",
        srcs = ["@io_bazel_rules_go//go/tools/builders:builder_srcs"],
        exec_compatible_with = exec_compatible_with,
        ldflags = "-X main.rulesGoStdlibPrefix={}".format(RULES_GO_STDLIB_PREFIX),
        # The .exe suffix is required on Windows and harmless on other platforms.
        # Output attributes are not configurable, so we use it everywhere.
        out_pack = "pack.exe",
        sdk = ":go_sdk",
    )

    non_go_reset_target(
        name = "builder_reset",
        dep = ":builder",
    )

    non_go_reset_target(
        name = "pack_reset",
        dep = ":pack.exe",
    )

    # TODO(jayconrod): Gazelle depends on this file directly. This dependency
    # should be broken, and this rule should be folded into go_sdk.
    package_list(
        name = "package_list",
        srcs = package_list_srcs,
        out = "packages.txt",
        root_file = "ROOT",
    )

    declare_go_toolchains(
        builder = ":builder_reset",
        exec_goos = goos,
        pack = ":pack_reset",
        sdk = ":go_sdk",
    )

    native.filegroup(
        name = "files",
        srcs = files_srcs,
    )

    native.exports_files(
        native.glob([
            "lib/time/zoneinfo.zip",
            # wasm support files including wasm_exec.js
            # for GOOS=js GOARCH=wasm
            # located in misc/wasm/ (Go 1.23 and earlier)
            # or lib/wasm/ (Go 1.24 and later)
            "*/wasm/**",
        ]),
        visibility = ["//visibility:public"],
    )
