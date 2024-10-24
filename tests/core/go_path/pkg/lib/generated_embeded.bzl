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
    "go_context",
)
load(
    "//go/private:context.bzl",
    "new_go_info",
)

def _gen_library_impl(ctx):
    go = go_context(ctx)
    libname = getattr(ctx.attr, "libname")
    src = go.actions.declare_file(ctx.label.name + ".go")

    embedsrcs = getattr(ctx.attr, "embedsrcs", [])

    lines = [
        "package " + libname,
        "",
        'import _ "embed"',
        "",
    ]

    i = 0
    for e in embedsrcs:
        for f in e.files.to_list():
            lines.extend([
                "//go:embed {}".format(f.basename),
                "var embeddedSource{} string".format(i),
            ])
            i += 1

    ctx.actions.write(src, "\n".join(lines))

    go_info = new_go_info(go, ctx.attr, generated_srcs = [src])
    archive = go.archive(go, go_info)
    return [
        go_info,
        archive,
        DefaultInfo(files = depset([archive.data.file])),
    ]

generated_embeded = rule(
    _gen_library_impl,
    attrs = {
        "importpath": attr.string(mandatory = True),
        "_go_context_data": attr.label(
            default = "//:go_context_data",
        ),
        "srcs": attr.label_list(
            allow_files = True,
        ),
        "embedsrcs": attr.label_list(
            allow_files = True,
        ),
        "libname": attr.string(default = "lib"),
    },
    toolchains = ["@io_bazel_rules_go//go:toolchain"],
)
