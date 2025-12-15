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

load(
    "@io_bazel_rules_go//go:def.bzl",
    "go_binary",
    "go_context",
)
load(
    "//go/private:context.bzl",
    "new_go_info",
)

_PREFIX = "/".join(["abcdefgh"[i] * 100 for i in range(7)]) + "/"

def _gen_library_impl(ctx):
    go = go_context(ctx)
    src = go.actions.declare_file(ctx.label.name + ".go")
    go.actions.write(src, "package " + ctx.label.name + "\n")

    go_info = new_go_info(go, ctx.attr, generated_srcs = [src])
    archive = go.archive(go, go_info)
    return [
        go_info,
        archive,
        DefaultInfo(files = depset([archive.data.file])),
    ]

_gen_library = rule(
    _gen_library_impl,
    attrs = {
        "importpath": attr.string(mandatory = True),
        "_go_context_data": attr.label(
            default = "//:go_context_data",
        ),
    },
    toolchains = ["@io_bazel_rules_go//go:toolchain"],
)

def _gen_main_src_impl(ctx):
    src = ctx.actions.declare_file(ctx.label.name + ".go")
    lines = [
        "package main",
        "",
        "import (",
    ]
    for i in range(ctx.attr.n):
        lines.append('\t_ "{}many_deps{}"'.format(_PREFIX, i))
    lines.extend([
        ")",
        "",
        "func main() {}",
    ])
    ctx.actions.write(src, "\n".join(lines))
    return [DefaultInfo(files = depset([src]))]

_gen_main_src = rule(
    _gen_main_src_impl,
    attrs = {
        "n": attr.int(mandatory = True),
    },
)

def many_deps(name, **kwargs):
    deps = []
    n = 200
    for i in range(n):
        lib_name = "many_deps" + str(i)
        _gen_library(
            name = lib_name,
            importpath = _PREFIX + lib_name,
            visibility = ["//visibility:private"],
        )
        deps.append(lib_name)
    _gen_main_src(
        name = "many_deps_src",
        n = n,
    )
    go_binary(
        name = name,
        srcs = [":many_deps_src"],
        deps = deps,
        **kwargs
    )
