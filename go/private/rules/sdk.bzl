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
    "//go/private:providers.bzl",
    "GoSDK",
)

def _go_sdk_impl(ctx):
    package_list = ctx.file.package_list
    if package_list == None:
        package_list = ctx.actions.declare_file("packages.txt")
        _build_package_list(ctx, ctx.files.srcs, ctx.file.root_file, package_list)
    return [
        DefaultInfo(
            files = depset(
                [ctx.file.go] +
                ctx.files.libs +
                ctx.files.headers +
                ctx.files.srcs +
                ctx.files.tools,
            ),
        ),
        GoSDK(
            goos = ctx.attr.goos,
            goarch = ctx.attr.goarch,
            experiments = ",".join(ctx.attr.experiments),
            toolchain_experiments = ",".join(ctx.attr.toolchain_experiments),
            root_file = ctx.file.root_file,
            package_list = package_list,
            libs = depset(ctx.files.libs),
            headers = depset(ctx.files.headers),
            srcs = depset(ctx.files.srcs),
            tools = depset(ctx.files.tools),
            go = ctx.executable.go,
            version = ctx.attr.version,
        ),
    ]

go_sdk = rule(
    _go_sdk_impl,
    attrs = {
        "goos": attr.string(
            mandatory = True,
            doc = "The host OS the SDK was built for",
        ),
        "goarch": attr.string(
            mandatory = True,
            doc = "The host architecture the SDK was built for",
        ),
        "experiments": attr.string_list(
            mandatory = False,
            doc = "Go experiments to enable via GOEXPERIMENT",
        ),
        "toolchain_experiments": attr.string_list(
            mandatory = False,
            doc = "Go experiments to enable via GOEXPERIMENT for toolchain tools",
        ),
        "root_file": attr.label(
            mandatory = True,
            allow_single_file = True,
            doc = "A file in the SDK root directory. Used to determine GOROOT.",
        ),
        "package_list": attr.label(
            allow_single_file = True,
            doc = ("A text file containing a list of packages in the " +
                   "standard library that may be imported."),
        ),
        "libs": attr.label_list(
            # allow_files is not set to [".a"] because that wouldn't allow
            # for zero files to be present, as is the case in Go 1.20+.
            # See also https://github.com/bazelbuild/bazel/issues/7516
            allow_files = True,
            doc = ("Pre-compiled .a files for the standard library, " +
                   "built for the execution platform"),
        ),
        "headers": attr.label_list(
            allow_files = [".h"],
            doc = (".h files from pkg/include that may be included in " +
                   "assembly sources"),
        ),
        "srcs": attr.label_list(
            allow_files = True,
            doc = "Source files for packages in the standard library",
        ),
        "tools": attr.label_list(
            allow_files = True,
            cfg = "exec",
            doc = ("List of executable files in the SDK built for " +
                   "the execution platform, excluding the go binary"),
        ),
        "go": attr.label(
            mandatory = True,
            allow_single_file = True,
            executable = True,
            cfg = "exec",
            doc = "The go binary",
        ),
        "version": attr.string(
            doc = "The version of the Go SDK.",
        ),
    },
    doc = ("Collects information about a Go SDK. The SDK must have a normal " +
           "GOROOT directory structure."),
    provides = [GoSDK],
)

def _package_list_impl(ctx):
    _build_package_list(ctx, ctx.files.srcs, ctx.file.root_file, ctx.outputs.out)
    return [DefaultInfo(files = depset([ctx.outputs.out]))]

package_list = rule(
    _package_list_impl,
    attrs = {
        "srcs": attr.label_list(
            allow_files = True,
            doc = "Source files for packages in the standard library",
        ),
        "root_file": attr.label(
            mandatory = True,
            allow_single_file = True,
            doc = "A file in the SDK root directory. Used to determine GOROOT.",
        ),
        "out": attr.output(
            mandatory = True,
            doc = "File to write. Must be 'packages.txt'.",
            # Gazelle depends on this file directly. It has to be an output
            # attribute because Bazel has no other way of knowing what rule
            # produces this file.
            # TODO(jayconrod): Update Gazelle and simplify this.
        ),
    },
)

def _build_package_list(ctx, srcs, root_file, out):
    packages = {}
    src_dir = root_file.dirname + "/src/"
    for src in srcs:
        pkg_src_dir = src.dirname
        if not pkg_src_dir.startswith(src_dir):
            continue
        pkg_name = pkg_src_dir[len(src_dir):]
        packages[pkg_name] = None
    content = "\n".join(sorted(packages.keys())) + "\n"
    ctx.actions.write(out, content)
