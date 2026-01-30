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
    "asm_exts",
    "cgo_exts",
    "go_exts",
    "syso_exts",
)
load(
    "//go/private:context.bzl",
    "CGO_ATTRS",
    "CGO_FRAGMENTS",
    "CGO_TOOLCHAINS",
    "go_context",
    "new_go_info",
)
load(
    "//go/private:providers.bzl",
    "GoInfo",
)
load(
    "//go/private/rules:transition.bzl",
    "non_go_transition",
)

def _go_library_impl(ctx):
    """Implements the go_library() rule."""
    go = go_context(
        ctx,
        include_deprecated_properties = False,
        importpath = ctx.attr.importpath,
        importmap = ctx.attr.importmap,
        importpath_aliases = ctx.attr.importpath_aliases,
        embed = ctx.attr.embed,
        go_context_data = ctx.attr._go_context_data,
    )

    go_info = new_go_info(go, ctx.attr)
    archive = go.archive(go, go_info)
    validation_output = archive.data._validation_output
    nogo_diagnostics = archive.data._nogo_diagnostics

    return [
        go_info,
        archive,
        DefaultInfo(
            files = depset([archive.data.export_file]),
        ),
        coverage_common.instrumented_files_info(
            ctx,
            source_attributes = ["srcs"],
            dependency_attributes = ["data", "deps", "embed", "embedsrcs"],
            extensions = ["go"],
        ),
        OutputGroupInfo(
            cgo_exports = archive.cgo_exports,
            compilation_outputs = [archive.data.file],
            nogo_fix = [nogo_diagnostics] if nogo_diagnostics else [],
            _validation = [validation_output] if validation_output else [],
        ),
    ]

go_library = rule(
    _go_library_impl,
    attrs = {
        "data": attr.label_list(
            allow_files = True,
            cfg = non_go_transition,
            doc = """
            List of files needed by this rule at run-time.
            This may include data files needed or other programs that may be executed.
            The [bazel] package may be used to locate run files; they may appear in different places
            depending on the operating system and environment. See [data dependencies] for more information on data files.
            """,
        ),
        "srcs": attr.label_list(
            allow_files = go_exts + asm_exts + cgo_exts + syso_exts,
            cfg = non_go_transition,
            doc = """
            The list of Go source files that are compiled to create the package.
            Only `.go`, `.s`, and `.syso` files are permitted, unless the `cgo` attribute is set,
            in which case, `.c .cc .cpp .cxx .h .hh .hpp .hxx .inc .m .mm` files are also permitted.
            Files may be filtered at build time using Go [build constraints].
            """,
        ),
        "deps": attr.label_list(
            providers = [GoInfo],
            doc = """
            List of Go libraries this package imports directly.
            These may be `go_library` rules or compatible rules with the [GoInfo] provider.
            """,
        ),
        "importpath": attr.string(
            doc = """
            The source import path of this library. Other libraries can import this library using this path.
            This must either be specified in `go_library` or inherited from one of the libraries in `embed`.
            """,
        ),
        "importmap": attr.string(
            doc = """
            The actual import path of this library. By default, this is `importpath`. This is mostly only visible to the compiler and linker,
            but it may also be seen in stack traces. This must be unique among packages passed to the linker.
            It may be set to something different than `importpath` to prevent conflicts between multiple packages
            with the same path (for example, from different vendor directories).
            """,
        ),
        "importpath_aliases": attr.string_list(),  # experimental, undocumented
        "embed": attr.label_list(
            providers = [GoInfo],
            doc = """
            List of Go libraries whose sources should be compiled together with this package's sources.
            Labels listed here must name `go_library`, `go_proto_library`, or other compatible targets with
            the [GoInfo] provider. Embedded libraries must have the same `importpath` as the embedding library.
            At most one embedded library may have `cgo = True`, and the embedding library may not also have `cgo = True`.
            See [Embedding] for more information.
            """,
        ),
        "embedsrcs": attr.label_list(
            allow_files = True,
            cfg = non_go_transition,
            doc = """
            The list of files that may be embedded into the compiled package using `//go:embed`
            directives. All files must be in the same logical directory or a subdirectory as source files.
            All source files containing `//go:embed` directives must be in the same logical directory.
            It's okay to mix static and generated source files and static and generated embeddable files.
            """,
        ),
        "gc_goopts": attr.string_list(
            doc = """
            List of flags to add to the Go compilation command when using the gc compiler.
            Subject to ["Make variable"] substitution and [Bourne shell tokenization].
            """,
        ),
        "x_defs": attr.string_dict(
            doc = """
            Map of defines to add to the go link command. See [Defines and stamping] for examples of how to use these.
            """,
        ),
        "cgo": attr.bool(
            doc = """
            If `True`, the package may contain [cgo] code, and `srcs` may contain C, C++, Objective-C, and Objective-C++ files
            and non-Go assembly files. When cgo is enabled, these files will be compiled with the C/C++ toolchain and
            included in the package. Note that this attribute does not force cgo to be enabled. Cgo is enabled for
            non-cross-compiling builds when a C/C++ toolchain is configured.
            """,
        ),
        "cdeps": attr.label_list(
            cfg = non_go_transition,
            doc = """
            List of other libraries that the c code depends on.
            This can be anything that would be allowed in [cc_library deps] Only valid if `cgo = True`.
            """,
        ),
        "cppopts": attr.string_list(
            doc = """
            List of flags to add to the C/C++ preprocessor command.
            Subject to ["Make variable"] substitution and [Bourne shell tokenization].
            Only valid if `cgo = True`.
            """,
        ),
        "copts": attr.string_list(
            doc = """
            List of flags to add to the C compilation command.
            Subject to ["Make variable"] substitution and [Bourne shell tokenization]. Only valid if `cgo = True`.
            """,
        ),
        "cxxopts": attr.string_list(
            doc = """
            List of flags to add to the C++ compilation command.
            Subject to ["Make variable"] substitution and [Bourne shell tokenization]. Only valid if `cgo = True`.
            """,
        ),
        "clinkopts": attr.string_list(
            doc = """
            List of flags to add to the C link command.
            Subject to ["Make variable"] substitution and [Bourne shell tokenization]. Only valid if `cgo = True`.
            """,
        ),
        "_go_context_data": attr.label(default = "//:go_context_data"),
        "_allowlist_function_transition": attr.label(
            default = "@bazel_tools//tools/allowlists/function_transition_allowlist",
        ),
    } | CGO_ATTRS,
    fragments = CGO_FRAGMENTS,
    toolchains = [GO_TOOLCHAIN] + CGO_TOOLCHAINS,
    provides = [GoInfo],
    doc = """This builds a Go library from a set of source files that are all part of
    the same package.

    ***Note:*** For targets generated by Gazelle, `name` is typically the last component of the path,
    or `go_default_library`, with the old naming convention.

    **Providers:**
    <ul>
      <li>[GoInfo]</li>
      <li>[GoArchive]</li>
    </ul>
    """,
)

# See docs/go/core/rules.md#go_library for full documentation.

def _go_tool_library_impl(ctx):
    """Implements the go_tool_library() rule."""
    go = go_context(ctx, include_deprecated_properties = False)

    go_info = new_go_info(go, ctx.attr)
    archive = go.archive(go, go_info)

    return [
        go_info,
        archive,
    ]

go_tool_library = rule(
    _go_tool_library_impl,
    attrs = {
        "data": attr.label_list(allow_files = True),
        "srcs": attr.label_list(allow_files = True),
        "deps": attr.label_list(providers = [GoInfo]),
        "importpath": attr.string(),
        "importmap": attr.string(),
        "embed": attr.label_list(providers = [GoInfo]),
        "gc_goopts": attr.string_list(),
        "x_defs": attr.string_dict(),
        "_go_config": attr.label(default = "//:go_config"),
        "_cgo_context_data": attr.label(default = "//:cgo_context_data_proxy"),
        "_stdlib": attr.label(default = "//:stdlib"),
    } | CGO_ATTRS,
    fragments = CGO_FRAGMENTS,
    toolchains = [GO_TOOLCHAIN] + CGO_TOOLCHAINS,
)
# This is used instead of `go_library` for dependencies of the `nogo` rule and
# packages that are depended on implicitly by code generated within the Go rules.
# This avoids a bootstrapping problem.

# See docs/go/core/rules.md#go_tool_library for full documentation.
