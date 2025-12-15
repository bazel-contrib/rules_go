# Copyright 2018 The Bazel Authors. All rights reserved.
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

load("//go/private:providers.bzl", "GoStdLib")

def _force_rebuild_transition_impl(settings, attr):
    return {"//go/config:race": True}

force_rebuild_transition = transition(
    implementation = _force_rebuild_transition_impl,
    inputs = ["//go/config:race"],
    outputs = ["//go/config:race"],
)

def _stdlib_files_impl(ctx):
    # When an outgoing transition (aka split transition) is used,
    # ctx.attr._stdlib is a list of Target.
    stdlib = ctx.attr._stdlib[0][GoStdLib]
    libs = stdlib.libs
    runfiles = ctx.runfiles(transitive_files = libs)
    return [DefaultInfo(
        files = depset([stdlib._list_json], transitive = [libs]),
        runfiles = runfiles,
    )]

stdlib_files = rule(
    implementation = _stdlib_files_impl,
    attrs = {
        "_stdlib": attr.label(
            default = "@io_bazel_rules_go//:stdlib",
            providers = [GoStdLib],
            cfg = force_rebuild_transition,
        ),
        "_allowlist_function_transition": attr.label(
            default = "@bazel_tools//tools/allowlists/function_transition_allowlist",
        ),
    },
)
