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
    "go_context",
    "new_go_info",
)
load(
    "//go/private:mode.bzl",
    "LINKMODES",
    "LINKMODES_EXECUTABLE",
    "LINKMODE_C_ARCHIVE",
    "LINKMODE_C_SHARED",
    "LINKMODE_PLUGIN",
    "LINKMODE_SHARED",
)
load(
    "//go/private:providers.bzl",
    "GoInfo",
    "GoSDK",
)
load(
    "//go/private/rules:transition.bzl",
    "go_transition",
)

_EMPTY_DEPSET = depset([])

def _include_path(hdr):
    if not hdr.root.path:
        fail("Expected hdr to be a generated file, got source file: " + hdr.path)

    root_relative_path = hdr.path[len(hdr.root.path + "/"):]
    if not root_relative_path.startswith("external/"):
        return hdr.root.path

    # All headers should be includeable via a path relative to their repository
    # root, regardless of whether the repository is external or not. If it is,
    # we thus need to append "external/<external repo name>" to the path.
    return "/".join([hdr.root.path] + root_relative_path.split("/")[0:2])

def new_cc_import(
        go,
        hdrs = _EMPTY_DEPSET,
        defines = _EMPTY_DEPSET,
        local_defines = _EMPTY_DEPSET,
        dynamic_library = None,
        static_library = None,
        alwayslink = False,
        linkopts = []):
    return CcInfo(
        compilation_context = cc_common.create_compilation_context(
            defines = defines,
            local_defines = local_defines,
            headers = hdrs,
            includes = depset([_include_path(hdr) for hdr in hdrs.to_list()]),
        ),
        linking_context = cc_common.create_linking_context(
            linker_inputs = depset([
                cc_common.create_linker_input(
                    owner = go.label,
                    libraries = depset([
                        cc_common.create_library_to_link(
                            actions = go.actions,
                            cc_toolchain = go.cgo_tools.cc_toolchain,
                            feature_configuration = go.cgo_tools.feature_configuration,
                            dynamic_library = dynamic_library,
                            static_library = static_library,
                            alwayslink = alwayslink,
                        ),
                    ]),
                    user_link_flags = depset(linkopts),
                ),
            ]),
        ),
    )

def _go_cc_aspect_impl(target, ctx):
    if GoInfo not in target:
        return []

    deps = (
        getattr(ctx.rule.attr, "cdeps", []) +
        getattr(ctx.rule.attr, "deps", []) +
        getattr(ctx.rule.attr, "embed", [])
    )
    return [
        cc_common.merge_cc_infos(cc_infos = [dep[CcInfo] for dep in deps]),
    ]

_go_cc_aspect = aspect(
    implementation = _go_cc_aspect_impl,
    attr_aspects = ["deps", "embed"],
)

def _go_binary_impl(ctx):
    """go_binary_impl emits actions for compiling and linking a go executable."""
    go = go_context(
        ctx,
        include_deprecated_properties = False,
        importpath = ctx.attr.importpath,
        embed = ctx.attr.embed,
        # It's a list because it is transitioned.
        go_context_data = ctx.attr._go_context_data[0],
        goos = ctx.attr.goos,
        goarch = ctx.attr.goarch,
    )

    is_main = go.mode.linkmode not in (LINKMODE_SHARED, LINKMODE_PLUGIN)
    go_info = new_go_info(
        go,
        ctx.attr,
        importable = False,
        is_main = is_main,
    )
    name = ctx.attr.basename
    if not name:
        name = ctx.label.name
    executable = None
    if ctx.attr.out:
        # Use declare_file instead of attr.output(). When users set output files
        # directly, Bazel warns them not to use the same name as the rule, which is
        # the common case with go_binary.
        executable = ctx.actions.declare_file(ctx.attr.out)
    archive, executable, runfiles = go.binary(
        go,
        name = name,
        source = go_info,
        gc_linkopts = gc_linkopts(ctx),
        version_file = ctx.version_file,
        info_file = ctx.info_file,
        executable = executable,
    )
    validation_output = archive.data._validation_output

    providers = [
        archive,
        OutputGroupInfo(
            cgo_exports = archive.cgo_exports,
            compilation_outputs = [archive.data.file],
            _validation = [validation_output] if validation_output else [],
        ),
    ]

    if go.mode.linkmode in LINKMODES_EXECUTABLE:
        env = {}
        for k, v in ctx.attr.env.items():
            env[k] = ctx.expand_location(v, ctx.attr.data)
        providers.append(RunEnvironmentInfo(environment = env))

        # The executable is automatically added to the runfiles.
        providers.append(DefaultInfo(
            files = depset([executable]),
            runfiles = runfiles,
            executable = executable,
        ))
    else:
        # Workaround for https://github.com/bazelbuild/bazel/issues/15043
        # As of Bazel 5.1.1, native rules do not pick up the "files" of a data
        # dependency's DefaultInfo, only the "data_runfiles". Since transitive
        # non-data dependents should not pick up the executable as a runfile
        # implicitly, the deprecated "default_runfiles" and "data_runfiles"
        # constructor parameters have to be used.
        providers.append(DefaultInfo(
            files = depset([executable]),
            default_runfiles = runfiles,
            data_runfiles = runfiles.merge(ctx.runfiles([executable])),
        ))

    # If the binary's linkmode is c-archive or c-shared, expose CcInfo
    if go.cgo_tools and go.mode.linkmode in (LINKMODE_C_ARCHIVE, LINKMODE_C_SHARED):
        cc_import_kwargs = {
            "linkopts": {
                "darwin": [],
                "ios": [],
                "windows": ["-mthreads"],
            }.get(go.mode.goos, ["-pthread"]),
        }
        cgo_exports = archive.cgo_exports.to_list()
        if cgo_exports:
            header = ctx.actions.declare_file("{}.h".format(name))
            ctx.actions.symlink(
                output = header,
                target_file = cgo_exports[0],
            )
            cc_import_kwargs["hdrs"] = depset([header])
        if go.mode.linkmode == LINKMODE_C_SHARED:
            cc_import_kwargs["dynamic_library"] = executable
        elif go.mode.linkmode == LINKMODE_C_ARCHIVE:
            cc_import_kwargs["static_library"] = executable
            cc_import_kwargs["alwayslink"] = True

        cc_infos = [new_cc_import(go, **cc_import_kwargs)]
        cc_infos.extend([dep[CcInfo] for dep in ctx.attr.cdeps + ctx.attr.deps + ctx.attr.embed if CcInfo in dep])

        ccinfo = cc_common.merge_cc_infos(cc_infos = cc_infos)
        providers.append(ccinfo)

    return providers

def _go_binary_kwargs(go_cc_aspects = []):
    return {
        "implementation": _go_binary_impl,
        "attrs": {
            "srcs": attr.label_list(
                allow_files = go_exts + asm_exts + cgo_exts + syso_exts,
                doc = """The list of Go source files that are compiled to create the package.
                Only `.go`, `.s`, and `.syso` files are permitted, unless the `cgo`
                attribute is set, in which case,
                `.c .cc .cpp .cxx .h .hh .hpp .hxx .inc .m .mm`
                files are also permitted. Files may be filtered at build time
                using Go [build constraints].
                """,
            ),
            "data": attr.label_list(
                allow_files = True,
                doc = """List of files needed by this rule at run-time. This may include data files
                needed or other programs that may be executed. The [bazel] package may be
                used to locate run files; they may appear in different places depending on the
                operating system and environment. See [data dependencies] for more
                information on data files.
                """,
            ),
            "deps": attr.label_list(
                providers = [GoInfo],
                aspects = go_cc_aspects,
                doc = """List of Go libraries this package imports directly.
                These may be `go_library` rules or compatible rules with the [GoInfo] provider.
                """,
                cfg = go_transition,
            ),
            "embed": attr.label_list(
                providers = [GoInfo],
                aspects = go_cc_aspects,
                doc = """List of Go libraries whose sources should be compiled together with this
                binary's sources. Labels listed here must name `go_library`,
                `go_proto_library`, or other compatible targets with the [GoInfo] provider.
                Embedded libraries must all have the same `importpath`,
                which must match the `importpath` for this `go_binary` if one is
                specified. At most one embedded library may have `cgo = True`, and the
                embedding binary may not also have `cgo = True`. See [Embedding] for
                more information.
                """,
                cfg = go_transition,
            ),
            "embedsrcs": attr.label_list(
                allow_files = True,
                doc = """The list of files that may be embedded into the compiled package using
                `//go:embed` directives. All files must be in the same logical directory
                or a subdirectory as source files. All source files containing `//go:embed`
                directives must be in the same logical directory. It's okay to mix static and
                generated source files and static and generated embeddable files.
                """,
            ),
            "env": attr.string_dict(
                doc = """Environment variables to set when the binary is executed with bazel run.
                The values (but not keys) are subject to
                [location expansion](https://docs.bazel.build/versions/main/skylark/macros.html) but not full
                [make variable expansion](https://docs.bazel.build/versions/main/be/make-variables.html).
                """,
            ),
            "importpath": attr.string(
                doc = """The import path of this binary. Binaries can't actually be imported, but this
                may be used by [go_path] and other tools to report the location of source
                files. This may be inferred from embedded libraries.
                """,
            ),
            "gc_goopts": attr.string_list(
                doc = """List of flags to add to the Go compilation command when using the gc compiler.
                Subject to ["Make variable"] substitution and [Bourne shell tokenization].
                """,
            ),
            "gc_linkopts": attr.string_list(
                doc = """List of flags to add to the Go link command when using the gc compiler.
                Subject to ["Make variable"] substitution and [Bourne shell tokenization].
                """,
            ),
            "x_defs": attr.string_dict(
                doc = """Map of defines to add to the go link command.
                See [Defines and stamping] for examples of how to use these.
                """,
            ),
            "basename": attr.string(
                doc = """The basename of this binary. The binary
                basename may also be platform-dependent: on Windows, we add an .exe extension.
                """,
            ),
            "out": attr.string(
                doc = """Sets the output filename for the generated executable. When set, `go_binary`
                will write this file without mode-specific directory prefixes, without
                linkmode-specific prefixes like "lib", and without platform-specific suffixes
                like ".exe". Note that without a mode-specific directory prefix, the
                output file (but not its dependencies) will be invalidated in Bazel's cache
                when changing configurations.
                """,
            ),
            "cgo": attr.bool(
                doc = """If `True`, the package may contain [cgo] code, and `srcs` may contain
                C, C++, Objective-C, and Objective-C++ files and non-Go assembly files.
                When cgo is enabled, these files will be compiled with the C/C++ toolchain
                and included in the package. Note that this attribute does not force cgo
                to be enabled. Cgo is enabled for non-cross-compiling builds when a C/C++
                toolchain is configured.
                """,
            ),
            "cdeps": attr.label_list(
                doc = """The list of other libraries that the c code depends on.
                This can be anything that would be allowed in [cc_library deps]
                Only valid if `cgo` = `True`.
                """,
            ),
            "cppopts": attr.string_list(
                doc = """List of flags to add to the C/C++ preprocessor command.
                Subject to ["Make variable"] substitution and [Bourne shell tokenization].
                Only valid if `cgo` = `True`.
                """,
            ),
            "copts": attr.string_list(
                doc = """List of flags to add to the C compilation command.
                Subject to ["Make variable"] substitution and [Bourne shell tokenization].
                Only valid if `cgo` = `True`.
                """,
            ),
            "cxxopts": attr.string_list(
                doc = """List of flags to add to the C++ compilation command.
                Subject to ["Make variable"] substitution and [Bourne shell tokenization].
                Only valid if `cgo` = `True`.
                """,
            ),
            "clinkopts": attr.string_list(
                doc = """List of flags to add to the C link command.
                Subject to ["Make variable"] substitution and [Bourne shell tokenization].
                Only valid if `cgo` = `True`.
                """,
            ),
            "pure": attr.string(
                default = "auto",
                doc = """Controls whether cgo source code and dependencies are compiled and linked,
                similar to setting `CGO_ENABLED`. May be one of `on`, `off`,
                or `auto`. If `auto`, pure mode is enabled when no C/C++
                toolchain is configured or when cross-compiling. It's usually better to
                control this on the command line with
                `--@io_bazel_rules_go//go/config:pure`. See [mode attributes], specifically
                [pure].
                """,
            ),
            "static": attr.string(
                default = "auto",
                doc = """Controls whether a binary is statically linked. May be one of `on`,
                `off`, or `auto`. Not available on all platforms or in all
                modes. It's usually better to control this on the command line with
                `--@io_bazel_rules_go//go/config:static`. See [mode attributes],
                specifically [static].
                """,
            ),
            "race": attr.string(
                default = "auto",
                doc = """Controls whether code is instrumented for race detection. May be one of
                `on`, `off`, or `auto`. Not available when cgo is
                disabled. In most cases, it's better to control this on the command line with
                `--@io_bazel_rules_go//go/config:race`. See [mode attributes], specifically
                [race].
                """,
            ),
            "msan": attr.string(
                default = "auto",
                doc = """Controls whether code is instrumented for memory sanitization. May be one of
                `on`, `off`, or `auto`. Not available when cgo is
                disabled. In most cases, it's better to control this on the command line with
                `--@io_bazel_rules_go//go/config:msan`. See [mode attributes], specifically
                [msan].
                """,
            ),
            "gotags": attr.string_list(
                doc = """Enables a list of build tags when evaluating [build constraints]. Useful for
                conditional compilation.
                """,
            ),
            "goos": attr.string(
                default = "auto",
                doc = """Forces a binary to be cross-compiled for a specific operating system. It's
                usually better to control this on the command line with `--platforms`.

                This disables cgo by default, since a cross-compiling C/C++ toolchain is
                rarely available. To force cgo, set `pure` = `off`.

                See [Cross compilation] for more information.
                """,
            ),
            "goarch": attr.string(
                default = "auto",
                doc = """Forces a binary to be cross-compiled for a specific architecture. It's usually
                better to control this on the command line with `--platforms`.

                This disables cgo by default, since a cross-compiling C/C++ toolchain is
                rarely available. To force cgo, set `pure` = `off`.

                See [Cross compilation] for more information.
                """,
            ),
            "linkmode": attr.string(
                default = "auto",
                values = ["auto"] + LINKMODES,
                doc = """Determines how the binary should be built and linked. This accepts some of
                the same values as `go build -buildmode` and works the same way.
                <br><br>
                <ul>
                <li>`auto` (default): Controlled by `//go/config:linkmode`, which defaults to `normal`.</li>
                <li>`normal`: Builds a normal executable with position-dependent code.</li>
                <li>`pie`: Builds a position-independent executable.</li>
                <li>`plugin`: Builds a shared library that can be loaded as a Go plugin. Only supported on platforms that support plugins.</li>
                <li>`c-shared`: Builds a shared library that can be linked into a C program.</li>
                <li>`c-archive`: Builds an archive that can be linked into a C program.</li>
                </ul>
                """,
            ),
            "pgoprofile": attr.label(
                allow_files = True,
                doc = """Provides a pprof file to be used for profile guided optimization when compiling go targets.
                A pprof file can also be provided via `--@io_bazel_rules_go//go/config:pgoprofile=<label of a pprof file>`.
                Profile guided optimization is only supported on go 1.20+.
                See https://go.dev/doc/pgo for more information.
                """,
                default = "//go/config:empty",
            ),
            "_go_context_data": attr.label(default = "//:go_context_data", cfg = go_transition),
            "_allowlist_function_transition": attr.label(
                default = "@bazel_tools//tools/allowlists/function_transition_allowlist",
            ),
        },
        "toolchains": [GO_TOOLCHAIN],
        "doc": """This builds an executable from a set of source files,
        which must all be in the `main` package. You can run the binary with
        `bazel run`, or you can build it with `bazel build` and run it directly.<br><br>
        ***Note:*** `name` should be the same as the desired name of the generated binary.<br><br>
        **Providers:**
        <ul>
          <li>[GoArchive]</li>
        </ul>
        """,
    }

go_binary = rule(executable = True, **_go_binary_kwargs())
go_non_executable_binary = rule(executable = False, **_go_binary_kwargs(
    go_cc_aspects = [_go_cc_aspect],
))

def _go_tool_binary_impl(ctx):
    # Keep in mind that the actions registered by this rule may not be
    # sandboxed, so care must be taken to make them hermetic, for example by
    # preventing `go build` from searching for go.mod or downloading a
    # different toolchain version.
    #
    # A side effect of the usage of GO111MODULE below is that the absolute
    # path to the sources is included in the buildid, which would make the
    # resulting binary non-reproducible. We thus need to blank it out.
    # https://github.com/golang/go/blob/583d750fa119d504686c737be6a898994b674b69/src/cmd/go/internal/load/pkg.go#L1764-L1766
    # https://github.com/golang/go/blob/583d750fa119d504686c737be6a898994b674b69/src/cmd/go/internal/work/exec.go#L284

    sdk = ctx.attr.sdk[GoSDK]
    name = ctx.label.name
    if sdk.goos == "windows":
        name += ".exe"

    out = ctx.actions.declare_file(name)
    if sdk.goos == "windows":
        # Using pre-declared directory for temporary output as there is no safe
        # way under Windows to create unique temporary dir.
        gotmp = ctx.actions.declare_directory("gotmp")
        cmd = """@echo off
set GOMAXPROCS=1
set GOCACHE=%cd%\\{gotmp}\\gocache
set GOPATH=%cd%"\\{gotmp}\\gopath
set GOTOOLCHAIN=local
set GO111MODULE=off
{go} build -o {out} -trimpath -ldflags \"-buildid='' {ldflags}\" {srcs}
set GO_EXIT_CODE=%ERRORLEVEL%
RMDIR /S /Q "{gotmp}"
MKDIR "{gotmp}"
exit /b %GO_EXIT_CODE%
""".format(
            gotmp = gotmp.path.replace("/", "\\"),
            go = sdk.go.path.replace("/", "\\"),
            out = out.path,
            srcs = " ".join([f.path for f in ctx.files.srcs]),
            ldflags = ctx.attr.ldflags,
        )
        bat = ctx.actions.declare_file(name + ".bat")
        ctx.actions.write(
            output = bat,
            content = cmd,
        )
        ctx.actions.run(
            executable = bat,
            tools = depset(
                ctx.files.srcs + [sdk.go],
                transitive = [sdk.headers, sdk.srcs, sdk.tools],
            ),
            toolchain = None,
            outputs = [out, gotmp],
            mnemonic = "GoToolchainBinaryBuild",
        )
    else:
        # Note: GOPATH is needed for Go 1.16.
        cmd = """
GOTMP=$(mktemp -d)
trap "rm -rf \"$GOTMP\"" EXIT
GOMAXPROCS=1 \
GOCACHE="$GOTMP"/gocache \
GOPATH="$GOTMP"/gopath \
GOTOOLCHAIN=local \
GO111MODULE=off \
{go} build -o {out} -trimpath -ldflags '-buildid="" {ldflags}' {srcs}
""".format(
            go = sdk.go.path,
            out = out.path,
            srcs = " ".join([f.path for f in ctx.files.srcs]),
            ldflags = ctx.attr.ldflags,
        )
        ctx.actions.run_shell(
            command = cmd,
            tools = depset(
                ctx.files.srcs + [sdk.go],
                transitive = [sdk.headers, sdk.srcs, sdk.libs, sdk.tools],
            ),
            toolchain = None,
            outputs = [out],
            mnemonic = "GoToolchainBinaryBuild",
        )

    return [DefaultInfo(
        files = depset([out]),
        executable = out,
    )]

go_tool_binary = rule(
    implementation = _go_tool_binary_impl,
    attrs = {
        "srcs": attr.label_list(
            allow_files = True,
            doc = "Source files for the binary. Must be in 'package main'.",
        ),
        "sdk": attr.label(
            mandatory = True,
            providers = [GoSDK],
            doc = "The SDK containing tools and libraries to build this binary",
        ),
        "ldflags": attr.string(
            doc = "Raw value to pass to go build via -ldflags without tokenization",
        ),
    },
    executable = True,
    doc = """Used instead of go_binary for executables used in the toolchain.

go_tool_binary depends on tools and libraries that are part of the Go SDK.
It does not depend on other toolchains. It can only compile binaries that
just have a main package and only depend on the standard library and don't
require build constraints.
""",
)

def gc_linkopts(ctx):
    gc_linkopts = [
        ctx.expand_make_variables("gc_linkopts", f, {})
        for f in ctx.attr.gc_linkopts
    ]
    return gc_linkopts
