# Copyright 2019 The Bazel Authors. All rights reserved.
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

load("//go/private:common.bzl", "GO_TOOLCHAIN_LABEL", "SUPPORTS_PATH_MAPPING_REQUIREMENT")
load(
    "//go/private:mode.bzl",
    "link_mode_arg",
)
load("//go/private/actions:utils.bzl", "quote_opts")

def _archive(v):
    importpaths = [v.data.importpath]
    importpaths.extend(v.data.importpath_aliases)
    return "{}={}={}".format(
        ":".join(importpaths),
        v.data.importmap,
        v.data.export_file.path if v.data.export_file else v.data.file.path,
    )

def _facts(v):
    facts_file = v.data.facts_file
    if not facts_file:
        return None
    importpaths = [v.data.importpath]
    importpaths.extend(v.data.importpath_aliases)
    return "{}={}={}".format(
        ":".join(importpaths),
        v.data.importmap,
        facts_file.path,
    )

def _embedroot_arg(src):
    return src.root.path

def _embedlookupdir_arg(src):
    root_relative = src.dirname[len(src.root.path):]
    if root_relative.startswith("/"):
        root_relative = root_relative[1:]
    return root_relative

def emit_compilepkg(
        go,
        sources = None,
        cover = None,
        embedsrcs = [],
        importpath = "",
        importmap = "",
        archives = [],
        cgo = False,
        cgo_inputs = depset(),
        cppopts = [],
        copts = [],
        cxxopts = [],
        objcopts = [],
        objcxxopts = [],
        clinkopts = [],
        out_lib = None,
        out_export = None,
        out_facts = None,
        out_nogo_log = None,
        out_nogo_validation = None,
        out_nogo_fix_tmp = None,
        out_nogo_fix = None,
        nogo = None,
        out_cgo_export_h = None,
        gc_goopts = [],
        testfilter = None,  # TODO: remove when test action compiles packages
        recompile_internal_deps = [],
        is_external_pkg = False):
    """Compiles a complete Go package."""
    if sources == None:
        fail("sources is a required parameter")
    if out_lib == None:
        fail("out_lib is a required parameter")
    if bool(nogo) != bool(out_facts):
        fail("nogo must be specified if and only if out_facts is specified")
    if bool(nogo) != bool(out_nogo_log):
        fail("nogo must be specified if and only if out_nogo_log is specified")
    if bool(nogo) != bool(out_nogo_validation):
        fail("nogo must be specified if and only if out_nogo_validation is specified")
    if bool(nogo) != bool(out_nogo_fix_tmp):
        fail("nogo must be specified if and only if out_nogo_fix_tmp is specified")
    if bool(nogo) != bool(out_nogo_fix):
        fail("nogo must be specified if and only if out_nogo_fix is specified")

    if cover and go.coverdata:
        archives = archives + [go.coverdata]

    sdk = go.sdk
    inputs_direct = (sources + embedsrcs + [sdk.package_list] +
                     [archive.data.export_file for archive in archives])
    inputs_transitive = [sdk.headers, sdk.tools, go.stdlib.libs]
    outputs = [out_lib, out_export]

    args = go.builder_args(go, "compilepkg", use_path_mapping = True)
    args.add_all(sources, before_each = "-src")
    args.add_all(embedsrcs, before_each = "-embedsrc", expand_directories = False)
    args.add_all(
        sources + [out_lib] + embedsrcs,
        map_each = _embedroot_arg,
        before_each = "-embedroot",
        uniquify = True,
        expand_directories = False,
    )
    args.add_all(
        sources + [out_lib],
        map_each = _embedlookupdir_arg,
        before_each = "-embedlookupdir",
        uniquify = True,
        expand_directories = False,
    )

    cover_mode = None
    if cover and go.coverdata:
        if go.mode.race:
            cover_mode = "atomic"
        else:
            cover_mode = "set"
        args.add("-cover_mode", cover_mode)
        args.add("-cover_format", go.mode.cover_format)
        args.add_all(cover, before_each = "-cover")

    args.add_all(archives, before_each = "-arc", map_each = _archive)
    if recompile_internal_deps:
        args.add_all(recompile_internal_deps, before_each = "-recompile_internal_deps")
    if importpath:
        args.add("-importpath", importpath)
    else:
        args.add("-importpath", go.label.name)
    if importmap:
        args.add("-p", importmap)
    args.add("-package_list", sdk.package_list)

    args.add("-lo", out_lib)
    args.add("-o", out_export)
    if out_cgo_export_h:
        args.add("-cgoexport", out_cgo_export_h)
        outputs.append(out_cgo_export_h)
    if testfilter:
        args.add("-testfilter", testfilter)

    link_mode_flag = link_mode_arg(go.mode)

    gc_flags = gc_goopts + go.mode.gc_goopts
    if go.mode.race:
        gc_flags.append("-race")
    if go.mode.msan:
        gc_flags.append("-msan")
    if go.mode.debug:
        gc_flags.extend(["-N", "-l"])
    gc_flags.extend(go.toolchain.flags.compile)
    if link_mode_flag:
        gc_flags.append(link_mode_flag)
    args.add("-gcflags", quote_opts(gc_flags))

    if link_mode_flag:
        args.add("-asmflags", link_mode_flag)

    # cgo and the linker action don't support path mapping yet
    # TODO: Remove the second condition after https://github.com/bazelbuild/bazel/pull/21921.
    if cgo or "local" in go._ctx.attr.tags:
        # cgo doesn't support path mapping yet
        env = go.env
        execution_requirements = {}
    else:
        env = go.env_for_path_mapping
        execution_requirements = SUPPORTS_PATH_MAPPING_REQUIREMENT
    cgo_go_srcs_for_nogo = None
    if cgo:
        if nogo:
            cgo_go_srcs_for_nogo = go.declare_directory(go, path = out_lib.basename + ".cgo")
            outputs.append(cgo_go_srcs_for_nogo)
            args.add("-cgo_go_srcs", cgo_go_srcs_for_nogo.path)
        inputs_transitive.append(cgo_inputs)
        inputs_transitive.append(go.cc_toolchain_files)
        env["CC"] = go.cgo_tools.c_compiler_path
        if cppopts:
            args.add("-cppflags", quote_opts(cppopts))
        if copts:
            args.add("-cflags", quote_opts(copts))
        if cxxopts:
            args.add("-cxxflags", quote_opts(cxxopts))
        if objcopts:
            args.add("-objcflags", quote_opts(objcopts))
        if objcxxopts:
            args.add("-objcxxflags", quote_opts(objcxxopts))
        if clinkopts:
            args.add("-ldflags", quote_opts(clinkopts))

    if go.mode.pgoprofile:
        args.add("-pgoprofile", go.mode.pgoprofile)
        inputs_direct.append(go.mode.pgoprofile)

    go.actions.run(
        inputs = depset(inputs_direct, transitive = inputs_transitive),
        outputs = outputs,
        mnemonic = "GoCompilePkgExternal" if is_external_pkg else "GoCompilePkg",
        executable = go.toolchain._builder,
        arguments = [args],
        env = env,
        toolchain = GO_TOOLCHAIN_LABEL,
        execution_requirements = execution_requirements,
    )

    if nogo:
        _run_nogo(
            go,
            sources = sources,
            cgo_go_srcs = cgo_go_srcs_for_nogo,
            importpath = importpath,
            importmap = importmap,
            archives = archives,
            recompile_internal_deps = recompile_internal_deps,
            cover_mode = cover_mode,
            testfilter = testfilter,
            out_facts = out_facts,
            out_log = out_nogo_log,
            out_validation = out_nogo_validation,
            out_nogo_fix_tmp = out_nogo_fix_tmp,
            out_nogo_fix = out_nogo_fix,
            nogo = nogo,
        )

def _run_nogo(
        go,
        *,
        sources,
        cgo_go_srcs,
        importpath,
        importmap,
        archives,
        recompile_internal_deps,
        cover_mode,
        testfilter,
        out_facts,
        out_log,
        out_validation,
        out_nogo_fix_tmp,
        out_nogo_fix,
        nogo):
    """Runs nogo on Go source files, including those generated by cgo."""
    sdk = go.sdk

    inputs_direct = (sources + [nogo, sdk.package_list] +
                     [archive.data.facts_file for archive in archives if archive.data.facts_file] +
                     [archive.data.export_file for archive in archives])
    inputs_transitive = [sdk.tools, sdk.headers, go.stdlib.libs]
    outputs = [out_facts, out_log, out_nogo_fix_tmp]

    args = go.builder_args(go, "nogo", use_path_mapping = True)
    args.add_all(sources, before_each = "-src")
    if cgo_go_srcs:
        inputs_direct.append(cgo_go_srcs)
        args.add_all([cgo_go_srcs], before_each = "-ignore_src")
    if cover_mode:
        args.add("-cover_mode", cover_mode)
    if recompile_internal_deps:
        args.add_all(recompile_internal_deps, before_each = "-recompile_internal_deps")
    args.add_all(archives, before_each = "-arc", map_each = _archive)
    if importpath:
        args.add("-importpath", importpath)
    else:
        args.add("-importpath", go.label.name)
    if importmap:
        args.add("-p", importmap)
    args.add("-package_list", sdk.package_list)
    if testfilter:
        args.add("-testfilter", testfilter)

    args.add_all(archives, before_each = "-facts", map_each = _facts)
    args.add("-out_facts", out_facts)
    args.add("-out_log", out_log)
    args.add("-out_fix", out_nogo_fix_tmp)
    args.add("-nogo", nogo)

    # This action runs nogo and produces the facts files for downstream nogo actions.
    # It is important that this action doesn't fail if nogo produces findings, which allows users
    # to get the nogo findings for all targets with --keep_going rather than stopping at the first
    # target with findings.
    # If nogo fails for any other reason, the action still fails, which allows users to debug their
    # analyzers with --sandbox_debug. Users can set debug = True on the nogo target to have it fail
    # on findings to get the same debugging experience as with other failures.
    go.actions.run(
        inputs = depset(inputs_direct, transitive = inputs_transitive),
        outputs = outputs,
        mnemonic = "RunNogo",
        executable = go.toolchain._builder,
        arguments = [args],
        env = go.env_for_path_mapping,
        toolchain = GO_TOOLCHAIN_LABEL,
        execution_requirements = SUPPORTS_PATH_MAPPING_REQUIREMENT,
        progress_message = "Running nogo on %{label}",
    )

    # This is a separate action that produces the validation output registered with Bazel. It
    # prints any nogo findings and, crucially, fails if there are any findings. This is necessary
    # to actually fail the build on nogo findings, which RunNogo doesn't do.
    validation_args = go.actions.args()
    validation_args.add("nogovalidation")
    validation_args.add(out_validation)
    validation_args.add(out_log)
    validation_args.add(out_nogo_fix_tmp)
    validation_args.add(out_nogo_fix)

    go.actions.run(
        inputs = [out_log, out_nogo_fix_tmp],
        outputs = [out_validation, out_nogo_fix],
        mnemonic = "ValidateNogo",
        executable = go.toolchain._builder,
        arguments = [validation_args],
        execution_requirements = SUPPORTS_PATH_MAPPING_REQUIREMENT,
        progress_message = "Validating nogo output for %{label}",
    )
