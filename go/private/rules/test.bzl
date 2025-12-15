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
    "@bazel_skylib//lib:structs.bzl",
    "structs",
)
load(
    "//go/private:common.bzl",
    "GO_TOOLCHAIN",
    "GO_TOOLCHAIN_LABEL",
    "SUPPORTS_PATH_MAPPING_REQUIREMENT",
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
    "//go/private:mode.bzl",
    "LINKMODES",
)
load(
    "//go/private:providers.bzl",
    "GoArchive",
    "GoInfo",
    "INFERRED_PATH",
)
load(
    "//go/private/rules:binary.bzl",
    "gc_linkopts",
)
load(
    "//go/private/rules:transition.bzl",
    "go_transition",
    "non_go_transition",
)

def _go_test_impl(ctx):
    """go_test_impl implements go testing.

    It emits an action to run the test generator, and then compiles the
    test into a binary."""

    go = go_context(
        ctx,
        include_deprecated_properties = False,
        importpath = ctx.attr.importpath,
        embed = ctx.attr.embed,
        go_context_data = ctx.attr._go_context_data,
        goos = ctx.attr.goos,
        goarch = ctx.attr.goarch,
    )

    validation_outputs = []
    nogo_diagnosticss = []

    # Compile the library to test with internal white box tests
    internal_go_info = new_go_info(
        go,
        ctx.attr,
        testfilter = "exclude",
    )
    internal_archive = go.archive(go, internal_go_info)
    if internal_archive.data._validation_output:
        validation_outputs.append(internal_archive.data._validation_output)
    if internal_archive.data._nogo_diagnostics:
        nogo_diagnosticss.append(internal_archive.data._nogo_diagnostics)
    go_srcs = [src for src in internal_go_info.srcs if src.extension == "go"]

    # Compile the library with the external black box tests
    external_go_info = new_go_info(
        go,
        struct(
            srcs = [struct(files = go_srcs)],
            data = ctx.attr.data,
            embedsrcs = [struct(files = internal_go_info.embedsrcs)],
            deps = internal_archive.direct + [internal_archive],
            x_defs = ctx.attr.x_defs,
        ),
        name = internal_go_info.name + "_test",
        importpath = internal_go_info.importpath + "_test",
        testfilter = "only",
    )
    external_go_info, internal_archive = _recompile_external_deps(go, external_go_info, internal_archive, [t.label for t in ctx.attr.embed])
    external_archive = go.archive(go, external_go_info, is_external_pkg = True)
    if external_archive.data._validation_output:
        validation_outputs.append(external_archive.data._validation_output)
    if external_archive.data._nogo_diagnostics:
        nogo_diagnosticss.append(external_archive.data._nogo_diagnostics)

    # now generate the main function
    repo_relative_rundir = ctx.attr.rundir or ctx.label.package or "."
    if ctx.label.repo_name:
        # The test is contained in an external repository (Label.repo_name is always the empty
        # string for the main repository, which is the canonical repository name of this repo).
        # The test runner cd's into the directory corresponding to the main repository, so walk up
        # and then down.
        run_dir = "../" + ctx.label.repo_name + "/" + repo_relative_rundir
    else:
        run_dir = repo_relative_rundir

    main_go = go.declare_file(go, path = "testmain.go")
    arguments = go.builder_args(go, "gentestmain")
    arguments.add("-output", main_go)
    if go.coverage_enabled:
        # Always use atomic mode as the "runtime/coverage" APIs require it
        # and test behavior should follow non-test behavior.
        arguments.add("-cover_mode", "atomic")
        arguments.add("-cover_format", go.mode.cover_format)
    arguments.add(
        # the l is the alias for the package under test, the l_test must be the
        # same with the test suffix
        "-import",
        "l=" + internal_go_info.importpath,
    )
    arguments.add(
        "-import",
        "l_test=" + external_go_info.importpath,
    )
    arguments.add("-pkgname", internal_go_info.importpath)
    arguments.add_all(go_srcs, before_each = "-src", format_each = "l=%s")

    ctx.actions.run(
        inputs = go_srcs,
        outputs = [main_go],
        mnemonic = "GoTestGenTest",
        executable = go.toolchain._builder,
        arguments = [arguments],
        toolchain = GO_TOOLCHAIN_LABEL,
        env = go.env_for_path_mapping,
        execution_requirements = SUPPORTS_PATH_MAPPING_REQUIREMENT,
    )

    test_gc_linkopts = gc_linkopts(ctx)
    if not go.mode.debug and go.mode.strip:
        # Disable symbol table and DWARF generation for test binaries.
        test_gc_linkopts.extend(["-s", "-w"])

    # Link in the run_dir global for bzltestutil.
    # We add "+initfirst/" to the package path so the package is initialized
    # before user code. See comment above the init function
    # in bzltestutil/init.go.
    test_gc_linkopts.extend(["-X", "+initfirst/github.com/bazelbuild/rules_go/go/tools/bzltestutil/chdir.RunDir=" + run_dir])

    # This is needed for the testing.Testing() function to work in go
    # 1.21+.  See
    # https://cs.opensource.google/go/go/+/refs/tags/go1.21.0:src/testing/testing.go;l=647-661
    # for more details.
    test_gc_linkopts.extend(["-X", "testing.testBinary=1"])

    # Now compile the test binary itself
    test_deps = external_archive.direct + [external_archive] + ctx.attr._testmain_additional_deps
    if go.coverage_enabled:
        test_deps.append(go.coverdata)
    test_go_info = new_go_info(
        go,
        struct(
            deps = test_deps,
        ),
        name = go.label.name + "~testmain",
        importpath = "testmain",
        pathtype = INFERRED_PATH,
        is_main = True,
        generated_srcs = [main_go],
        coverage_instrumented = False,
    )
    test_archive, executable, runfiles = go.binary(
        go,
        name = ctx.label.name,
        source = test_go_info,
        test_archives = [internal_archive.data],
        gc_linkopts = test_gc_linkopts,
        version_file = ctx.version_file,
        info_file = ctx.info_file,
    )

    env = {
        # The test binary uses this to decide
        # whether it was invoked by Bazel or directly.
        # If invoked directly, it will not change its working directory
        # to run_dir configured above.
        "GO_TEST_RUN_FROM_BAZEL": "1",
    }
    for k, v in ctx.attr.env.items():
        env[k] = ctx.expand_location(v, ctx.attr.data) if "$" in v else v

    run_environment_info = RunEnvironmentInfo(env, ctx.attr.env_inherit)

    # Bazel only looks for coverage data if the test target has an
    # InstrumentedFilesProvider. If the provider is found and at least one
    # source file is present, Bazel will set the COVERAGE_OUTPUT_FILE
    # environment variable during tests and will save that file to the build
    # events + test outputs.
    return [
        test_archive,
        DefaultInfo(
            files = depset([executable]),
            runfiles = runfiles,
            executable = executable,
        ),
        OutputGroupInfo(
            compilation_outputs = [
                internal_archive.data.file,
                external_archive.data.file,
            ],
            nogo_fix = nogo_diagnosticss,
            _validation = validation_outputs,
        ),
        coverage_common.instrumented_files_info(
            ctx,
            source_attributes = ["srcs"],
            dependency_attributes = ["data", "deps", "embed", "embedsrcs"],
            extensions = ["go"],
        ),
        run_environment_info,
    ]

_go_test_kwargs = {
    "cfg": go_transition,
    "implementation": _go_test_impl,
    "attrs": {
        "data": attr.label_list(
            allow_files = True,
            cfg = non_go_transition,
            doc = """List of files needed by this rule at run-time. This may include data files
            needed or other programs that may be executed. The [bazel] package may be
            used to locate run files; they may appear in different places depending on the
            operating system and environment. See [data dependencies] for more
            information on data files.
            """,
        ),
        "srcs": attr.label_list(
            allow_files = go_exts + asm_exts + cgo_exts + syso_exts,
            cfg = non_go_transition,
            doc = """The list of Go source files that are compiled to create the package.
            Only `.go`, `.s`, and `.syso` files are permitted, unless the `cgo`
            attribute is set, in which case,
            `.c .cc .cpp .cxx .h .hh .hpp .hxx .inc .m .mm`
            files are also permitted. Files may be filtered at build time
            using Go [build constraints].
            """,
        ),
        "deps": attr.label_list(
            providers = [GoInfo],
            doc = """List of Go libraries this test imports directly.
            These may be go_library rules or compatible rules with the [GoInfo] provider.
            """,
        ),
        "embed": attr.label_list(
            providers = [GoInfo],
            doc = """List of Go libraries whose sources should be compiled together with this
            package's sources. Labels listed here must name `go_library`,
            `go_proto_library`, or other compatible targets with the
            [GoInfo] provider. Embedded libraries must have the same `importpath` as
            the embedding library. At most one embedded library may have `cgo = True`,
            and the embedding library may not also have `cgo = True`. See [Embedding]
            for more information.
            """,
        ),
        "embedsrcs": attr.label_list(
            allow_files = True,
            cfg = non_go_transition,
            doc = """The list of files that may be embedded into the compiled package using
            `//go:embed` directives. All files must be in the same logical directory
            or a subdirectory as source files. All source files containing `//go:embed`
            directives must be in the same logical directory. It's okay to mix static and
            generated source files and static and generated embeddable files.
            """,
        ),
        "env": attr.string_dict(
            doc = """Environment variables to set for the test execution.
            The values (but not keys) are subject to
            [location expansion](https://docs.bazel.build/versions/main/skylark/macros.html) but not full
            [make variable expansion](https://docs.bazel.build/versions/main/be/make-variables.html).
            """,
        ),
        "env_inherit": attr.string_list(
            doc = """Environment variables to inherit from the external environment.
            """,
        ),
        "importpath": attr.string(
            doc = """The import path of this test. Tests can't actually be imported, but this
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
        "rundir": attr.string(
            doc = """ A directory to cd to before the test is run.
            This should be a path relative to the root directory of the
            repository in which the test is defined, which can be the main or an
            external repository.

            The default behaviour is to change to the relative path
            corresponding to the test's package, which replicates the normal
            behaviour of `go test` so it is easy to write compatible tests.

            Setting it to `.` makes the test behave the normal way for a bazel
            test, except that the working directory is always that of the test's
            repository, which is not necessarily the main repository.

            Note: If runfile symlinks are disabled (such as on Windows by
            default), the test will run in the working directory set by Bazel,
            which is the subdirectory of the runfiles directory corresponding to
            the main repository.
            """,
        ),
        "x_defs": attr.string_dict(
            doc = """Map of defines to add to the go link command.
            See [Defines and stamping] for examples of how to use these.
            """,
        ),
        "linkmode": attr.string(
            default = "auto",
            values = ["auto"] + LINKMODES,
            doc = """Determines how the binary should be built and linked. This accepts some of
            the same values as `go build -buildmode` and works the same way.
            <br><br>
            <ul>
            <li>`auto` (default): Controlled by `//go/config:linkmode`, which defaults to `pie` on supported platforms and `normal` elsewhere.</li>
            <li>`normal`: Builds a normal executable with position-dependent code.</li>
            <li>`pie`: Builds a position-independent executable.</li>
            <li>`plugin`: Builds a shared library that can be loaded as a Go plugin. Only supported on platforms that support plugins.</li>
            <li>`c-shared`: Builds a shared library that can be linked into a C program.</li>
            <li>`c-archive`: Builds an archive that can be linked into a C program.</li>
            </ul>
            """,
        ),
        "cgo": attr.bool(
            doc = """
            If `True`, the package may contain [cgo] code, and `srcs` may contain
            C, C++, Objective-C, and Objective-C++ files and non-Go assembly files.
            When cgo is enabled, these files will be compiled with the C/C++ toolchain
            and included in the package. Note that this attribute does not force cgo
            to be enabled. Cgo is enabled for non-cross-compiling builds when a C/C++
            toolchain is configured.
            """,
        ),
        "cdeps": attr.label_list(
            cfg = non_go_transition,
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
        "_go_context_data": attr.label(default = "//:go_context_data"),
        "_testmain_additional_deps": attr.label_list(
            providers = [GoInfo],
            default = ["//go/tools/bzltestutil"],
        ),
        # Required for Bazel to collect coverage of instrumented C/C++ binaries
        # executed by go_test.
        # This is just a shell script and thus cheap enough to depend on
        # unconditionally.
        "_collect_cc_coverage": attr.label(
            default = "@bazel_tools//tools/test:collect_cc_coverage",
            cfg = "exec",
        ),
        # Required for Bazel to merge coverage reports for Go and other
        # languages into a single report per test.
        # Using configuration_field ensures that the tool is only built when
        # run with bazel coverage, not with bazel test.
        "_lcov_merger": attr.label(
            default = configuration_field(fragment = "coverage", name = "output_generator"),
            cfg = "exec",
        ),
        "_allowlist_function_transition": attr.label(
            default = "@bazel_tools//tools/allowlists/function_transition_allowlist",
        ),
    } | CGO_ATTRS,
    "executable": True,
    "test": True,
    "fragments": CGO_FRAGMENTS,
    "toolchains": [GO_TOOLCHAIN] + CGO_TOOLCHAINS,
    "doc": """This builds a set of tests that can be run with `bazel test`.<br><br>
    To run all tests in the workspace, and print output on failure (the
    equivalent of `go test ./...`), run<br>
    ```
    bazel test --test_output=errors //...
    ```<br><br>
    To run a Go benchmark test, run<br>
    ```
    bazel run //path/to:test -- -test.bench=.
    ```<br><br>
    You can run specific tests by passing the `--test_filter=pattern
    <test_filter_>` argument to Bazel. You can pass arguments to tests by passing
    `--test_arg=arg <test_arg_>` arguments to Bazel, and you can set environment
    variables in the test environment by passing
    `--test_env=VAR=value <test_env_>`. You can terminate test execution after the first
    failure by passing the `--test_runner_fail_fast <test_runner_fail_fast_>` argument
    to Bazel. This is equivalent to passing `--test_arg=-failfast <test_arg_>`.<br><br>
    To write structured testlog information to Bazel's `XML_OUTPUT_FILE`, tests
    ran with `bazel test` execute using a wrapper. This functionality can be
    disabled by setting `GO_TEST_WRAP=0` in the test environment. Additionally,
    the testbinary can be invoked with `-test.v` by setting
    `GO_TEST_WRAP_TESTV=1` in the test environment; this will result in the
    `XML_OUTPUT_FILE` containing more granular data.<br><br>
    ***Note:*** To interoperate cleanly with old targets generated by [Gazelle], `name`
    should be `go_default_test` for internal tests and
    `go_default_xtest` for external tests. Gazelle now generates
    the name  based on the last component of the path. For example, a test
    in `//foo/bar` is named `bar_test`, and uses internal and external
    sources.
    """,
}

go_test = rule(**_go_test_kwargs)

def _recompile_external_deps(go, external_go_info, internal_archive, library_labels):
    """Recompiles some archives in order to split internal and external tests.

    go_test, like 'go test', splits tests into two separate archives: an
    internal archive ('package foo') and an external archive
    ('package foo_test'). The library under test is embedded into the internal
    archive. The external archive may import it and may depend on symbols
    defined in the internal test files.

    To avoid conflicts, the library under test must not be linked into the test
    binary, since the internal test archive embeds the same sources.
    Libraries imported by the external test that transitively import the
    library under test must be recompiled too, or the linker will complain that
    export data they were compiled with doesn't match the export data they
    are linked with.

    This function identifies which archives may need to be recompiled, then
    declares new output files and actions to recompile them. This is an
    unfortunately an expensive process requiring O(V+E) time and space in the
    size of the test's dependency graph for each test.

    Args:
        go: go object returned by go_context.
        external_go_info: GoInfo for the external archive.
        internal_archive: GoArchive for the internal archive.
        library_labels: labels for embedded libraries under test.

    Returns:
        external_source: recompiled GoInfo for the external archive. If no
            recompilation is needed, the original GoInfo is returned.
        internal_archive: recompiled GoArchive for the internal archive. If no
            recompilation is needed, the original GoInfo is returned.
    """

    # If no libraries are embedded in the internal archive, then nothing needs
    # to be recompiled.
    if not library_labels:
        return external_go_info, internal_archive

    # Build a map from labels to GoArchiveData.
    # If none of the librares embedded in the internal archive are in the
    # dependency graph, then nothing needs to be recompiled.
    arc_data_list = depset(transitive = [archive.transitive for archive in external_go_info.deps]).to_list()
    label_to_arc_data = {a.label: a for a in arc_data_list}
    if all([l not in label_to_arc_data for l in library_labels]):
        return external_go_info, internal_archive

    # Build a depth-first post-order list of dependencies starting with the
    # external archive. Each archive appears after its dependencies and before
    # its dependents.
    #
    # This is tricky because Starlark doesn't support recursion or while loops.
    # We simulate a while loop by iterating over a list of 2N elements where
    # N is the number of archives. Each archive is pushed onto the stack
    # twice: once before its dependencies are pushed, and once after.

    # dep_list is the post-order list of dependencies we're building.
    dep_list = []

    # stack is a stack of targets to process. We're done when it's empty.
    stack = [archive.data.label for archive in external_go_info.deps]

    # deps_pushed tracks the status of each target.
    # DEPS_UNPROCESSED means the target is on the stack, but its dependencies
    # are not.
    # Non-negative integers are the number of dependencies on the stack that
    # still need to be processed.
    # A target is on the stack if its status is DEPS_UNPROCESSED or 0.
    DEPS_UNPROCESSED = -1
    deps_pushed = {l: DEPS_UNPROCESSED for l in stack}

    # dependents maps labels to lists of known dependents. When a target is
    # processed, its dependents' deps_pushed count is deprecated.
    dependents = {l: [] for l in stack}

    # step is a list to iterate over to simulate a while loop. i tracks
    # iterations.
    step = [None] * (2 * len(arc_data_list))
    i = 0
    for _ in step:
        if len(stack) == 0:
            break
        i += 1

        label = stack.pop()
        if deps_pushed[label] == 0:
            # All deps have been added to dep_list. Append this target to the
            # list. If a dependent is not waiting for anything else, push
            # it back onto the stack.
            dep_list.append(label)
            for p in dependents.get(label, []):
                deps_pushed[p] -= 1
                if deps_pushed[p] == 0:
                    stack.append(p)
            continue

        # deps_pushed[label] == None, indicating we don't know whether this
        # targets dependencies have been processed. Other targets processed
        # earlier may depend on them.
        deps_pushed[label] = 0
        arc_data = label_to_arc_data[label]
        for c in arc_data._dep_labels:
            if c not in deps_pushed:
                # Dependency not seen yet; push it.
                stack.append(c)
                deps_pushed[c] = None
                deps_pushed[label] += 1
                dependents[c] = [label]
            elif deps_pushed[c] != 0:
                # Dependency pushed, not processed; wait for it.
                deps_pushed[label] += 1
                dependents[c].append(label)
        if deps_pushed[label] == 0:
            # No dependencies to wait for; push self.
            stack.append(label)
    if i != len(step):
        fail("assertion failed: iterated %d times instead of %d" % (i, len(step)))

    # Determine which dependencies need to be recompiled because they depend
    # on embedded libraries.
    need_recompile = {}
    for label in dep_list:
        arc_data = label_to_arc_data[label]
        need_recompile[label] = any([
            dep in library_labels or need_recompile[dep]
            for dep in arc_data._dep_labels
        ])

    # Recompile the internal archive without dependencies that need
    # recompilation. This breaks a cycle which occurs because the deps list
    # is shared between the internal and external archive. The internal archive
    # can't import anything that imports itself.
    internal_go_info = internal_archive.source

    internal_deps = []

    # Pass internal dependencies that need to be recompiled down to the builder to check if the internal archive
    # tries to import any of the dependencies. If there is, that means that there is a dependency cycle.
    need_recompile_deps = []
    for archive in internal_go_info.deps:
        dep_data = archive.data
        if not need_recompile[dep_data.label]:
            internal_deps.append(archive)
        else:
            need_recompile_deps.append(dep_data.importpath)

    x_defs = dict(internal_go_info.x_defs)
    x_defs.update(internal_archive.x_defs)
    attrs = structs.to_dict(internal_go_info)
    attrs["deps"] = internal_deps
    attrs["x_defs"] = x_defs
    internal_go_info = GoInfo(**attrs)
    internal_archive = go.archive(go, internal_go_info, _recompile_suffix = ".recompileinternal", recompile_internal_deps = need_recompile_deps)

    # Build a map from labels to possibly recompiled GoArchives.
    label_to_archive = {}
    i = 0
    for label in dep_list:
        i += 1

        # If this library is the internal archive, use the recompiled version.
        if label == internal_archive.data.label:
            label_to_archive[label] = internal_archive
            continue

        # If this is a library embedded into the internal test archive,
        # use the internal test archive instead.
        if label in library_labels:
            label_to_archive[label] = internal_archive
            continue

        # Create a stub GoInfo from the archive data.
        arc_data = label_to_arc_data[label]
        deps = [label_to_archive[d] for d in arc_data._dep_labels]
        go_info = GoInfo(
            name = arc_data.name,
            label = arc_data.label,
            importpath = arc_data.importpath,
            importmap = arc_data.importmap,
            importpath_aliases = arc_data.importpath_aliases,
            pathtype = arc_data.pathtype,
            testfilter = None,
            is_main = False,
            mode = go.mode,
            srcs = list(arc_data.srcs),
            cover = arc_data._cover,
            embedsrcs = list(arc_data._embedsrcs),
            x_defs = dict(arc_data._x_defs),
            deps = deps,
            gc_goopts = list(arc_data._gc_goopts),
            runfiles = arc_data.runfiles,
            cgo = arc_data._cgo,
            cdeps = list(arc_data._cdeps),
            cppopts = list(arc_data._cppopts),
            copts = list(arc_data._copts),
            cxxopts = list(arc_data._cxxopts),
            clinkopts = list(arc_data._clinkopts),
        )

        # If this archive needs to be recompiled, use go.archive.
        # Otherwise, create a stub GoArchive, using the original file.
        if need_recompile[label]:
            recompile_suffix = ".recompile%d" % i
            archive = go.archive(go, go_info, _recompile_suffix = recompile_suffix)
        else:
            archive = GoArchive(
                source = go_info,
                data = arc_data,
                direct = deps,
                libs = depset(direct = [arc_data.file], transitive = [a.libs for a in deps]),
                transitive = depset(direct = [arc_data], transitive = [a.transitive for a in deps]),
                x_defs = go_info.x_defs,
                cgo_deps = depset(transitive = [arc_data._cgo_deps] + [a.cgo_deps for a in deps]),
                cgo_exports = depset(transitive = [a.cgo_exports for a in deps]),
                runfiles = go_info.runfiles,
                mode = go.mode,
                _headers = internal_archive._headers,
            )
        label_to_archive[label] = archive

    # Finally, we need to replace external_go_info.deps with the recompiled
    # archives.
    attrs = structs.to_dict(external_go_info)
    attrs["deps"] = [label_to_archive[archive.data.label] for archive in external_go_info.deps]
    return GoInfo(**attrs), internal_archive
