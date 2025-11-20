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

load(
    "//go/private:common.bzl",
    "GO_TOOLCHAIN",
)
load(
    "//go/private:context.bzl",
    "CGO_ATTRS",
    "CGO_FRAGMENTS",
    "CGO_TOOLCHAINS",
    "go_context",
)
load(
    "//go/private:providers.bzl",
    "GoConfigInfo",
)
load(
    "//go/private/rules:transition.bzl",
    "go_stdlib_transition",
)

def _stdlib_impl(ctx):
    go = go_context(ctx, include_deprecated_properties = False)
    return go.toolchain.actions.stdlib(go)

stdlib = rule(
    implementation = _stdlib_impl,
    cfg = go_stdlib_transition,
    attrs = {
        "cgo_context_data": attr.label(),
        "_go_config": attr.label(
            default = "//:go_config",
            providers = [GoConfigInfo],
        ),
        "_allowlist_function_transition": attr.label(
            default = "@bazel_tools//tools/allowlists/function_transition_allowlist",
        ),
    } | CGO_ATTRS,
    doc = """stdlib builds the standard library for the target configuration
or uses the precompiled standard library from the SDK if it is suitable.""",
    fragments = CGO_FRAGMENTS,
    toolchains = [GO_TOOLCHAIN] + CGO_TOOLCHAINS,
)
