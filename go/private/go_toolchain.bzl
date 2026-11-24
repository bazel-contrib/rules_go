# Copyright 2016 The Bazel Go Rules Authors. All rights reserved.
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
"""
Toolchain rules used by go.
"""

load("//go/private:platforms.bzl", "PLATFORMS")
load("//go/private:providers.bzl", "GoSDK")
load("//go/private/actions:archive.bzl", "emit_archive")
load("//go/private/actions:binary.bzl", "emit_binary")
load("//go/private/actions:link.bzl", "emit_link")
load("//go/private/actions:stdlib.bzl", "emit_stdlib")

def _go_toolchain_impl(ctx):
    sdk = ctx.attr.sdk[GoSDK]
    cross_compile = ctx.attr.goos != sdk.goos or ctx.attr.goarch != sdk.goarch
    return [
        ctx.attr.sdk[DefaultInfo],
        platform_common.ToolchainInfo(
            # Public fields
            name = ctx.label.name,
            cross_compile = cross_compile,
            default_goos = ctx.attr.goos,
            default_goarch = ctx.attr.goarch,
            actions = struct(
                archive = emit_archive,
                binary = emit_binary,
                link = emit_link,
                stdlib = emit_stdlib,
            ),
            flags = struct(
                compile = (),
                link = ctx.attr.link_flags,
                link_cgo = ctx.attr.cgo_link_flags,
            ),
            sdk = sdk,

            # Internal fields -- may be read by emit functions.
            _builder = ctx.executable.builder,
            _pack = ctx.executable.pack,
            _covdata = ctx.executable.covdata,
        ),
    ]

go_toolchain = rule(
    _go_toolchain_impl,
    attrs = {
        # Minimum requirements to specify a toolchain
        "builder": attr.label(
            mandatory = True,
            cfg = "exec",
            executable = True,
            doc = "Tool used to execute most Go actions",
        ),
        "pack": attr.label(
            mandatory = True,
            cfg = "exec",
            executable = True,
            doc = "Tool used to pack object files into archives",
        ),
        "covdata": attr.label(
            cfg = "exec",
            executable = True,
            doc = """Tool used to decode coverage meta-data files. If absent,
            no baseline coverage is emitted for packages without tests.""",
        ),
        "goos": attr.string(
            mandatory = True,
            doc = "Default target OS",
        ),
        "goarch": attr.string(
            mandatory = True,
            doc = "Default target architecture",
        ),
        "sdk": attr.label(
            mandatory = True,
            providers = [GoSDK],
            cfg = "exec",
            doc = "The SDK this toolchain is based on",
        ),
        # Optional extras to a toolchain
        "link_flags": attr.string_list(
            doc = "Flags passed to the Go internal linker",
        ),
        "cgo_link_flags": attr.string_list(
            doc = "Flags passed to the external linker (if it is used)",
        ),
    },
    doc = "Defines a Go toolchain based on an SDK",
    provides = [platform_common.ToolchainInfo],
)

def declare_go_toolchains(exec_goos, sdk, builder, pack, covdata):
    """Declares go_toolchain targets for each platform."""
    for p in PLATFORMS:
        if p.cgo:
            # Don't declare separate toolchains for cgo_on / cgo_off.
            continue

        link_flags = []
        cgo_link_flags = []
        if exec_goos == "darwin":
            cgo_link_flags.extend(["-shared", "-Wl,-all_load"])
        elif exec_goos == "linux":
            cgo_link_flags.append("-Wl,-whole-archive")

        go_toolchain(
            name = "go_" + p.name + "-impl",
            goos = p.goos,
            goarch = p.goarch,
            sdk = sdk,
            builder = builder,
            pack = pack,
            covdata = covdata,
            link_flags = link_flags,
            cgo_link_flags = cgo_link_flags,
            tags = ["manual"],
            visibility = ["//visibility:public"],
        )
