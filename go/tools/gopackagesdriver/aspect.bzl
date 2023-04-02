# Copyright 2021 The Bazel Go Rules Authors. All rights reserved.
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
    "@bazel_skylib//lib:paths.bzl",
    "paths",
)
load(
    "//go/private:providers.bzl",
    "GoArchive",
    "GoStdLib",
)

GoPkgInfo = provider()

DEPS_ATTRS = [
    "deps",
    "embed",
]

PROTO_COMPILER_ATTRS = [
    "compiler",
    "compilers",
    "library",
]

def bazel_supports_canonical_label_literals():
    return str(Label("//:bogus")).startswith("@@")

def is_file_external(f):
    return f.owner.workspace_root != ""

def file_path(f):
    prefix = "__BAZEL_WORKSPACE__"
    if not f.is_source:
        prefix = "__BAZEL_EXECROOT__"
    elif is_file_external(f):
        prefix = "__BAZEL_OUTPUT_BASE__"
    return paths.join(prefix, f.path)

def _go_archive_to_pkg_json(ctx, name, archive):
    pkg_json_file = ctx.actions.declare_file(name + ".pkg.json")

    # Build the args in a config file.
    args = ctx.actions.args()
    args.add("--id", archive.data.label)
    args.add("--pkg-path", archive.data.importpath)
    args.add("--export-file", archive.data.export_file)

    # Will expand into GoFiles and OtherFiles.
    args.add_all("--orig-srcs", archive.data.orig_srcs, map_each=file_path)

    # Expands in CompiledGoFiles
    args.add_all("--data-srcs", archive.data.srcs, map_each=file_path)
    args.add("--output-file", pkg_json_file)
    args.use_param_file("@%s")

    inputs = archive.data.orig_srcs + archive.data.srcs
    ctx.actions.run(
        inputs = inputs,
        outputs = [pkg_json_file],
        executable = ctx.executable._archive_to_json,
        execution_requirements = {
            "local": "1"
        },
        mnemonic = "ArchiveToJSON",
        arguments = [args],
    )
    return pkg_json_file

def _go_pkg_info_aspect_impl(target, ctx):
    # Fetch the stdlib JSON file from the inner most target
    stdlib_json_file = None

    transitive_json_files = []
    transitive_export_files = []
    transitive_compiled_go_files = []

    for attr in DEPS_ATTRS + PROTO_COMPILER_ATTRS:
        deps = getattr(ctx.rule.attr, attr, []) or []

        # Some attrs are not iterable, ensure that deps is always iterable.
        if type(deps) != type([]):
            deps = [deps]

        for dep in deps:
            if GoPkgInfo in dep:
                pkg_info = dep[GoPkgInfo]
                transitive_json_files.append(pkg_info.pkg_json_files)
                transitive_compiled_go_files.append(pkg_info.compiled_go_files)
                transitive_export_files.append(pkg_info.export_files)

                # Fetch the stdlib json from the first dependency
                if not stdlib_json_file:
                    stdlib_json_file = pkg_info.stdlib_json_file

    pkg_json_files = []
    compiled_go_files = []
    export_files = []

    if GoArchive in target:
        archive = target[GoArchive]
        compiled_go_files.extend(archive.source.srcs)
        export_files.append(archive.data.export_file)
        pkg_json_files.append(_go_archive_to_pkg_json(ctx, archive.data.name, archive))

        if ctx.rule.kind == "go_test":
            for dep_archive in archive.direct:
                # find the archive containing the test sources
                if archive.data.label == dep_archive.data.label:
                    pkg_json_files.append(_go_archive_to_pkg_json(ctx, dep_archive.data.name, dep_archive))
                    compiled_go_files.extend(dep_archive.source.srcs)
                    export_files.append(dep_archive.data.export_file)
                    break

    # If there was no stdlib json in any dependencies, fetch it from the
    # current go_ node.
    if not stdlib_json_file:
        stdlib_json_file = ctx.attr._go_stdlib[GoStdLib]._list_json

    pkg_info = GoPkgInfo(
        stdlib_json_file = stdlib_json_file,
        pkg_json_files = depset(
            direct = pkg_json_files,
            transitive = transitive_json_files,
        ),
        compiled_go_files = depset(
            direct = compiled_go_files,
            transitive = transitive_compiled_go_files,
        ),
        export_files = depset(
            direct = export_files,
            transitive = transitive_export_files,
        ),
    )

    return [
        pkg_info,
        OutputGroupInfo(
            go_pkg_driver_json_file = pkg_info.pkg_json_files,
            go_pkg_driver_srcs = pkg_info.compiled_go_files,
            go_pkg_driver_export_file = pkg_info.export_files,
            go_pkg_driver_stdlib_json_file = depset([pkg_info.stdlib_json_file] if pkg_info.stdlib_json_file else []),
        ),
    ]

go_pkg_info_aspect = aspect(
    implementation = _go_pkg_info_aspect_impl,
    attr_aspects = DEPS_ATTRS + PROTO_COMPILER_ATTRS,
    attrs = {
        "_go_stdlib": attr.label(
            default = "//:stdlib",
        ),
        "_archive_to_json": attr.label(
            executable = True,
            cfg = "exec",
            default = "//go/tools/gopackagesdriver:archive_to_json",
        ),
    },
)
