# Copyright 2024 The Bazel Authors. All rights reserved.
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

# Helper that wraps --proto_compiler into a ProtoLangToolchainInfo for backwards
# compatibility with --noincompatible_enable_proto_toolchain_resolution.

load(
    "@com_google_protobuf//bazel/common:proto_lang_toolchain_info.bzl",
    "ProtoLangToolchainInfo",
)
load(
    "//go/private/rules:transition.bzl",
    "non_go_tool_transition",
)

def _legacy_proto_toolchain_impl(ctx):
    return [
        ProtoLangToolchainInfo(
            proto_compiler = ctx.attr._protoc.files_to_run,
        ),
    ]

legacy_proto_toolchain = rule(
    implementation = _legacy_proto_toolchain_impl,
    cfg = non_go_tool_transition,
    attrs = {
        "_protoc": attr.label(
            default = configuration_field(fragment = "proto", name = "proto_compiler"),
        ),
        "_allowlist_function_transition": attr.label(
            default = "@bazel_tools//tools/allowlists/function_transition_allowlist",
        ),
    },
    fragments = ["proto"],
)
