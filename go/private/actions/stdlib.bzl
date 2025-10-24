# Copyright 2019 The Bazel Go Rules Authors. All rights reserved.
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
    "COVERAGE_OPTIONS_DENYLIST",
    "GO_TOOLCHAIN_LABEL",
    "SUPPORTS_PATH_MAPPING_REQUIREMENT",
)
load(
    "//go/private:context.bzl",
    "new_go_info",
)
load(
    "//go/private:mode.bzl",
    "LINKMODE_NORMAL",
    "extldflags_from_cc_toolchain",
    "link_mode_arg",
)
load(
    "//go/private:providers.bzl",
    "GoStdLib",
)
load("//go/private:sdk.bzl", "parse_version")
load("//go/private/actions:utils.bzl", "quote_opts")

def emit_stdlib(go):
    """Returns a standard library for the target configuration.

    If the precompiled standard library is suitable, it will be returned.
    Otherwise, the standard library will be compiled for the target.

    Returns:
        A list of providers containing GoInfo and GoStdLib.
    """
    go_info = new_go_info(go, {}, coverage_instrumented = False)
    stdlib = _sdk_stdlib(go) if _should_use_sdk_stdlib(go) else _build_stdlib(go)
    return [go_info, stdlib]

def _should_use_sdk_stdlib(go):
    version = parse_version(go.sdk.version)
    if version and version[0] <= 1 and version[1] <= 19 and go.sdk.experiments:
        # The precompiled stdlib shipped with 1.19 or below doesn't have experiments
        return False
    return (go.sdk.libs and  # go.sdk.libs is non-empty if sdk ships with precompiled .a files
            go.mode.goos == go.sdk.goos and
            go.mode.goarch == go.sdk.goarch and
            not go.mode.race and  # TODO(jayconrod): use precompiled race
            not go.mode.msan and
            not go.mode.pure and
            not go.mode.gc_goopts and
            go.mode.linkmode == LINKMODE_NORMAL)

def _build_stdlib_list_json(go):
    sdk = go.sdk

    out = go.declare_file(go, "stdlib.pkg.json")
    cache_dir = go.declare_directory(go, "gocache")
    args = go.builder_args(go, "stdliblist")
    args.add("-out", out)
    args.add_all("-cache", [cache_dir], expand_directories = False)
    if go.export_stdlib:
        args.add("-export", go.export_stdlib)

    inputs_direct = [sdk.go]
    inputs_transitive = [sdk.headers, sdk.srcs, sdk.libs, sdk.tools]
    if not go.mode.pure:
        inputs_transitive.append(go.cc_toolchain_files)

    go.actions.run(
        inputs = depset(inputs_direct, transitive = inputs_transitive),
        outputs = [out, cache_dir],
        mnemonic = "GoStdlibList",
        executable = go.toolchain._builder,
        arguments = [args],
        env = _build_env(go),
        toolchain = GO_TOOLCHAIN_LABEL,
        execution_requirements = SUPPORTS_PATH_MAPPING_REQUIREMENT,
    )
    return out, cache_dir

def _build_env(go):
    env = go.env

    if go.mode.pure:
        env.update({"CGO_ENABLED": "0"})
        return env

    # NOTE(#2545): avoid unnecessary dynamic link
    # go std library doesn't use C++, so should not have -lstdc++
    # Also drop coverage flags as nothing in the stdlib is compiled with
    # coverage - we disable it for all CGo code anyway.
    # NOTE(#3590): avoid forcing static linking.
    ldflags = [
        option
        for option in extldflags_from_cc_toolchain(go)
        if option not in ("-lstdc++", "-lc++", "-static") and option not in COVERAGE_OPTIONS_DENYLIST
    ]
    env.update({
        "CGO_ENABLED": "1",
        "CC": go.cgo_tools.c_compiler_path,
        "CGO_CFLAGS": " ".join(go.cgo_tools.c_compile_options),
        "CGO_LDFLAGS": " ".join(ldflags),
    })

    return env

def _sdk_stdlib(go):
    list_json, cache_dir = _build_stdlib_list_json(go)
    return GoStdLib(
        _list_json = list_json,
        cache_dir = depset([cache_dir]),
        libs = go.sdk.libs,
        root_file = go.sdk.root_file,
    )

def _dirname(file):
    return file.dirname

def _build_stdlib(go):
    pkg = go.declare_directory(go, path = "pkg")
    args = go.builder_args(go, "stdlib")

    # Use a file rather than pkg.dirname as the latter is just a string and thus
    # not subject to path mapping.
    args.add_all("-out", [pkg], map_each = _dirname, expand_directories = False)
    if go.mode.race:
        args.add("-race")
    if go.mode.msan:
        args.add("-msan")
    args.add("-package", "std")
    if not go.mode.pure:
        args.add("-package", "runtime/cgo")

    version = parse_version(go.sdk.version)
    if version and version[0] >= 1 and version[1] >= 20:
        # For bzltestutil's coverage support - `cmd/internal/cov` only introduced in go 1.20
        args.add("-package", "cmd/internal/cov")
        args.add("-package", "cmd/internal/bio")

    link_mode_flag = link_mode_arg(go.mode)
    if link_mode_flag:
        args.add(link_mode_flag)

    args.add("-gcflags", quote_opts(go.mode.gc_goopts))

    sdk = go.sdk
    inputs_direct = [sdk.go, sdk.package_list, sdk.root_file]
    inputs_transitive = [sdk.headers, sdk.srcs, sdk.tools, go.cc_toolchain_files]

    if go.mode.pgoprofile:
        args.add("-pgoprofile", go.mode.pgoprofile)
        inputs_direct.append(go.mode.pgoprofile)

    outputs = [pkg]
    go.actions.run(
        inputs = depset(direct = inputs_direct, transitive = inputs_transitive),
        outputs = outputs,
        mnemonic = "GoStdlib",
        executable = go.toolchain._builder,
        arguments = [args],
        env = _build_env(go),
        toolchain = GO_TOOLCHAIN_LABEL,
        execution_requirements = SUPPORTS_PATH_MAPPING_REQUIREMENT,
    )
    list_json, cache_dir = _build_stdlib_list_json(go)
    return GoStdLib(
        _list_json = list_json,
        libs = depset([pkg]),
        cache_dir = depset([cache_dir]),
        root_file = pkg,
    )
