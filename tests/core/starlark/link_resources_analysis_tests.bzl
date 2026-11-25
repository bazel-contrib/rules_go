# Copyright 2026 The Bazel Authors. All rights reserved.
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

"""Verify resource estimates passed to the Go linker action emitter."""

load("@bazel_skylib//lib:unittest.bzl", "analysistest", "asserts")
load("//go/private:common.bzl", "GO_TOOLCHAIN_LABEL")
load("//go/private/actions:link.bzl", "emit_link")

_LinkResourcesInfo = provider(fields = ["resources", "resource_type"])

def _record_action(ctx, calls, **kwargs):
    # Action inspection does not expose resource_set. Record the exact value
    # supplied by emit_link before registering the real action.
    resources = kwargs.get("resource_set")
    resource_type = type(resources)
    if resources != None and resource_type != "dict":
        resources = resources("linux", len(kwargs["inputs"].to_list()))
    calls.append(_LinkResourcesInfo(resources = resources, resource_type = resource_type))
    ctx.actions.run(**kwargs)

def _builder_args(go, verb):
    args = go._ctx.actions.args()
    args.add(verb)
    return args

def _link_resource_probe_impl(ctx):
    archive_file = ctx.actions.declare_file(ctx.label.name + ".a")
    imports = ctx.actions.declare_file(ctx.label.name + ".imports")
    builder = ctx.actions.declare_file(ctx.label.name + ".builder.sh")
    ctx.actions.write(archive_file, "")
    ctx.actions.write(imports, "")
    ctx.actions.write(builder, "#!/bin/sh\nexit 1\n", is_executable = True)
    data = struct(
        label = ctx.label,
        importpath = "example.com/main",
        importmap = "main",
        file = archive_file,
        _imports = imports,
        _package_metadata = None,
    )
    archive = struct(
        data = data,
        direct = [],
        transitive = depset([data]),
        x_defs = {},
        libs = depset([archive_file]),
        cgo_deps = depset(),
        cgo_link_inputs = depset(),
    )
    calls = []
    go = struct(
        _ctx = ctx,
        actions = struct(run = lambda **kwargs: _record_action(ctx, calls, **kwargs)),
        builder_args = _builder_args,
        tool_args = lambda go: ctx.actions.args(),
        mode = struct(
            linkmode = ctx.attr.linkmode,
            gc_linkopts = ctx.attr.mode_linkopts,
            pure = False,
            race = False,
            msan = False,
            stamp = False,
            strip = False,
            goos = "linux",
        ),
        cgo_tools = struct(
            ar_path = "",
            ld_executable_options = ctx.attr.toolchain_flags,
            ld_dynamic_lib_options = ctx.attr.dynamic_toolchain_flags,
            ld_executable_path = "/bin/false",
            ld_dynamic_lib_path = "/bin/false",
        ),
        static_link = False,
        coverage_enabled = False,
        coverdata = None,
        sdk = struct(package_list = imports, tools = depset(), version = "1.26.0"),
        stdlib = struct(libs = depset(), cgo_link_inputs = depset()),
        cc_toolchain_files = depset(),
        toolchain = struct(_builder = builder, flags = struct(link = [])),
        env = {},
    )
    emit_link(
        go,
        archive = archive,
        executable = ctx.actions.declare_file(ctx.label.name + ".bin"),
        gc_linkopts = ctx.attr.linkopts,
    )
    return calls

_link_resource_probe = rule(
    implementation = _link_resource_probe_impl,
    attrs = {
        "toolchain_flags": attr.string_list(),
        "dynamic_toolchain_flags": attr.string_list(),
        "linkopts": attr.string_list(),
        "mode_linkopts": attr.string_list(),
        "linkmode": attr.string(default = "normal"),
    },
    toolchains = [config_common.toolchain_type(GO_TOOLCHAIN_LABEL, mandatory = False)],
    _skylark_testable = True,
)

def _link_resources_analysis_test_impl(ctx):
    env = analysistest.begin(ctx)
    actions = [action for action in analysistest.target_actions(env) if action.mnemonic == "GoLink"]
    asserts.equals(env, 1, len(actions))
    info = analysistest.target_under_test(env)[_LinkResourcesInfo]
    expected_cpu = ctx.attr.expected_cpu
    if info.resource_type == "dict" and ctx.attr.expected_modern_cpu:
        expected_cpu = ctx.attr.expected_modern_cpu
    asserts.equals(env, {"cpu": expected_cpu, "memory": 512}, info.resources)
    if actions and ctx.attr.expected_flags:
        argv = actions[0].argv
        asserts.true(env, "-extldflags" in argv)
        if "-extldflags" in argv:
            asserts.equals(env, " ".join(ctx.attr.expected_flags), argv[argv.index("-extldflags") + 1])
    return analysistest.end(env)

_link_resources_analysis_test = analysistest.make(
    _link_resources_analysis_test_impl,
    attrs = {
        "expected_cpu": attr.int(mandatory = True),
        "expected_modern_cpu": attr.int(),
        "expected_flags": attr.string_list(),
    },
)

def link_resources_analysis_test_suite():
    cases = [
        ("default", {}, 1, 0, []),
        ("toolchain", {"toolchain_flags": ["-Wl,--threads=3"]}, 3, 0, ["-Wl,--threads=3"]),
        ("binary_override", {
            "toolchain_flags": ["-Wl,--threads=3"],
            "linkopts": ["-extldflags", "-Wl,--threads=5"],
        }, 5, 0, ["-Wl,--threads=3", "-Wl,--threads=5"]),
        ("mode_override", {
            "toolchain_flags": ["-Wl,--threads=3"],
            "linkopts": ["-extldflags", "-Wl,--threads=5"],
            "mode_linkopts": ["-extldflags", "-Wl,--threads=7"],
        }, 7, 0, ["-Wl,--threads=3", "-Wl,--threads=5", "-Wl,--threads=7"]),
        ("split_flags", {"linkopts": ["-extldflags", "-Wl", "--threads=6"]}, 6, 0, ["-Wl,--threads=6"]),
        ("large_count", {"toolchain_flags": ["-Wl,--threads=16"]}, 8, 16, ["-Wl,--threads=16"]),
        ("shared_library", {
            "toolchain_flags": ["-Wl,--threads=7"],
            "dynamic_toolchain_flags": ["-Wl,--threads=2"],
            "linkmode": "c-shared",
        }, 2, 0, ["-Wl,--threads=2"]),
    ]
    tests = []
    for name, attrs, cpu, modern_cpu, flags in cases:
        target = "link_resources_{}_probe".format(name)
        test = "link_resources_{}_analysis_test".format(name)
        _link_resource_probe(name = target, tags = ["manual"], **attrs)
        _link_resources_analysis_test(
            name = test,
            size = "small",
            target_under_test = ":" + target,
            expected_cpu = cpu,
            expected_modern_cpu = modern_cpu,
            expected_flags = flags,
        )
        tests.append(":" + test)
    native.test_suite(name = "link_resources_analysis_tests", tests = tests)
