# Copyright 2017 The Bazel Authors. All rights reserved.
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
    "new_go_info",
)
load(
    "//go/private:providers.bzl",
    "GoInfo",
)

def _go_source_impl(ctx):
    """Implements the go_source() rule."""
    go = go_context(ctx, include_deprecated_properties = False)
    go_info = new_go_info(go, ctx.attr)
    return [
        go_info,
        DefaultInfo(
            files = depset(go_info.srcs),
        ),
    ]

go_source = rule(
    implementation = _go_source_impl,
    attrs = {
        "data": attr.label_list(
            allow_files = True,
            doc = """List of files needed by this rule at run-time. This may include data files
            needed or other programs that may be executed. The [bazel] package may be
            used to locate run files; they may appear in different places depending on the
            operating system and environment. See [data dependencies] for more
            information on data files.
            """,
        ),
        "srcs": attr.label_list(
            allow_files = True,
            doc = """The list of Go source files that are compiled to create the package.
            The following file types are permitted: `.go, .c, .s, .syso, .S, .h`.
            The files may contain Go-style [build constraints].
            """,
        ),
        "deps": attr.label_list(
            providers = [GoInfo],
            doc = """List of Go libraries this source list imports directly.
            These may be go_library rules or compatible rules with the [GoInfo] provider.
            """,
        ),
        "embed": attr.label_list(
            providers = [GoInfo],
            doc = """List of Go libraries whose sources should be compiled together with this
            package's sources. Labels listed here must name `go_library`,
            `go_proto_library`, or other compatible targets with the [GoInfo]
            provider. Embedded libraries must have the same `importpath` as
            the embedding library. At most one embedded library may have `cgo = True`,
            and the embedding library may not also have `cgo = True`. See [Embedding]
            for more information.
            """,
        ),
        "gc_goopts": attr.string_list(
            doc = """List of flags to add to the Go compilation command when using the gc compiler.
            Subject to ["Make variable"] substitution and [Bourne shell tokenization].
            """,
        ),
        "_go_config": attr.label(default = "//:go_config"),
        "_cgo_context_data": attr.label(default = "//:cgo_context_data_proxy"),
    },
    toolchains = [GO_TOOLCHAIN],
    provides = [GoInfo],
    doc = """This declares a set of source files and related dependencies that can be embedded into one of the
    other rules.
    This is used as a way of easily declaring a common set of sources re-used in multiple rules.<br><br>
    **Providers:**
    <ul>
      <li>[GoInfo]</li>
    </ul>
    """,
)
# See docs/go/core/rules.md#go_source for full documentation.
