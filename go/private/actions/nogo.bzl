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
load("//go/private:context.bzl", "validate_nogo")

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

def emit_nogo(
        go,
        source,
        output_suffix = "",
        importpath = "",
        importmap = "",
        cgo_go_srcs = None,
        recompile_internal_deps = None):
    """Declares analysis outputs and registers nogo and validation actions."""
    nogo = go.nogo

    # Some targets have no nogo provider, or one without an executable.
    if nogo == None or nogo.executable == None or "no-nogo" in go._ctx.attr.tags:
        return struct(facts = None, diagnostics = None, validation = None)

    out_facts = go.declare_file(go, name = source.name, ext = output_suffix + ".facts")
    out_diagnostics = go.declare_directory(go, name = source.name, ext = output_suffix + "_nogo")
    out_validation = None
    if validate_nogo(go):
        out_validation = go.declare_file(go, name = source.name, ext = output_suffix + ".nogo")

    sources = source.srcs
    archives = source.deps
    if source.cover and go.coverdata:
        archives = archives + [go.coverdata]

    builder_args = go.builder_args(go)
    builder_args.add_all(sources, before_each = "-src")
    if source.cover and go.coverdata:
        builder_args.add("-cover_mode", "atomic")
    builder_args.add_all(archives, before_each = "-arc", map_each = _archive)
    if recompile_internal_deps:
        builder_args.add_all(recompile_internal_deps, before_each = "-recompile_internal_deps")
    builder_args.add("-importpath", importpath if importpath else go.label.name)
    if importmap:
        builder_args.add("-p", importmap)
    builder_args.add("-package_list", go.sdk.package_list)
    testfilter = getattr(source, "testfilter", None)
    if testfilter:
        builder_args.add("-testfilter", testfilter)

    go_version = go.sdk.version
    sdk = go.sdk

    inputs_direct = (sources + [sdk.package_list] +
                     [archive.data.facts_file for archive in archives if archive.data.facts_file] +
                     [archive.data.export_file for archive in archives])
    inputs_transitive = [sdk.tools, sdk.headers, go.stdlib.libs]
    outputs = [out_diagnostics, out_facts]

    nogo_args = go.tool_args(go)
    if cgo_go_srcs:
        inputs_direct.append(cgo_go_srcs)
        nogo_args.add_all([cgo_go_srcs], before_each = "-ignore_src")

    nogo_args.add_all(archives, before_each = "-facts", map_each = _facts)
    if not out_validation:
        # Since diagnostics are ignored, analyzers that don't generate facts can be skipped.
        nogo_args.add("-facts_only")
    nogo_args.add("-out_facts", out_facts)
    nogo_args.add_all("-out", [out_diagnostics], expand_directories = False)
    if go_version:
        # -go_version is the raw SDK version from go.sdk.version (for example
        # "1.24.3"), without the leading "go" prefix expected by go/types.
        # nogo_main.go normalizes it before type checking.
        nogo_args.add("-go_version", go_version)
    nogo_args.add("-nogo", nogo.executable)

    # This action runs nogo and produces the facts files for downstream nogo actions.
    # It is important that this action doesn't fail if nogo produces findings, which allows users
    # to get the nogo findings for all targets with --keep_going rather than stopping at the first
    # target with findings.
    # If nogo fails for any other reason, the action still fails, which allows users to debug their
    # analyzers with --sandbox_debug. Users can set debug = True on the nogo target to have it fail
    # on findings to get the same debugging experience as with other failures.
    go.actions.run(
        inputs = depset(inputs_direct, transitive = inputs_transitive),
        tools = [nogo],
        outputs = outputs,
        mnemonic = "RunNogo",
        executable = go.toolchain._builder,
        arguments = ["nogo", builder_args, nogo_args],
        env = go.env_for_path_mapping,
        toolchain = GO_TOOLCHAIN_LABEL,
        execution_requirements = SUPPORTS_PATH_MAPPING_REQUIREMENT,
        progress_message = "Running nogo on %{label}",
    )

    if out_validation:
        # This is a separate action that produces the validation output registered with Bazel. It
        # prints any nogo findings and, crucially, fails if there are any findings. This is necessary
        # to actually fail the build on nogo findings, which RunNogo doesn't do.
        validation_args = go.actions.args()
        validation_args.add("nogovalidation")
        validation_args.add(out_validation)
        validation_args.add_all([out_diagnostics], expand_directories = False)

        go.actions.run(
            inputs = [out_diagnostics],
            outputs = [out_validation],
            mnemonic = "ValidateNogo",
            executable = go.toolchain._builder,
            toolchain = GO_TOOLCHAIN_LABEL,
            arguments = [validation_args],
            execution_requirements = SUPPORTS_PATH_MAPPING_REQUIREMENT,
            progress_message = "Validating nogo output for %{label}",
        )

    return struct(facts = out_facts, diagnostics = out_diagnostics, validation = out_validation)
