# Copyright 2014 The Bazel Authors. All rights reserved.
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
    "go_context",
)

def _go_info_impl(ctx):
    go = go_context(ctx)
    report = go.declare_file(go, ext = ".txt")
    args = go.actions.args()
    args.add("-sdk", go.sdk.root_file.dirname)
    args.add("-out", report)
    go.actions.run(
        inputs = depset([go.sdk.go], transitive = [go.sdk.tools]),
        outputs = [report],
        mnemonic = "GoInfo",
        executable = ctx.executable._go_info,
        arguments = [args],
    )
    return [DefaultInfo(
        files = depset([report]),
        runfiles = ctx.runfiles([report]),
    )]

_go_info = rule(
    implementation = _go_info_impl,
    attrs = {
        "_go_info": attr.label(
            executable = True,
            cfg = "exec",
            default = "//go/tools/builders:info",
        ),
        "_go_context_data": attr.label(
            default = "//:go_context_data",
        ),
    },
    toolchains = [GO_TOOLCHAIN],
)

def go_info():
    _go_info(
        name = "go_info",
        visibility = ["//visibility:public"],
    )
