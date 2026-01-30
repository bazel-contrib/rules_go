# Copyright 2022 The Bazel Authors. All rights reserved.
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

load("@rules_cc//cc/common:cc_info.bzl", "CcInfo")
load(
    "//go/private:providers.bzl",
    "GoArchive",
)
load(
    "//go/private/rules:transition.bzl",
    "go_cross_transition",
)

def _is_windows(ctx):
    return ctx.configuration.host_path_separator == ";"

WINDOWS_ERR_SCRIPT = """
@echo off
>&2 echo {}
exit /b 1
"""
UNIX_ERR_SCRIPT = """
>&2 echo '{}'
exit 1
"""

def _error_script(ctx):
    errmsg = 'cannot run go_cross target "{}": underlying target "{}" is not executable'.format(
        ctx.attr.name,
        ctx.attr.target.label,
    )
    if _is_windows(ctx):
        error_script = ctx.actions.declare_file("fake_executable_for_bazel_run.bat")
        ctx.actions.write(error_script, WINDOWS_ERR_SCRIPT.format(errmsg), is_executable = True)
        return error_script

    error_script = ctx.actions.declare_file("fake_executable_for_bazel_run")
    ctx.actions.write(error_script, UNIX_ERR_SCRIPT.format(errmsg), is_executable = True)
    return error_script

def _go_cross_impl(ctx):
    old_default_info = ctx.attr.target[DefaultInfo]
    old_executable = old_default_info.files_to_run.executable

    if old_executable:
        # Bazel requires executable rules to created the executable themselves,
        # so we create a symlink in this rule so that it appears this rule created its executable.
        new_executable = ctx.actions.declare_file(ctx.attr.name)
        ctx.actions.symlink(output = new_executable, target_file = old_executable)
        new_default_info = DefaultInfo(
            files = depset([new_executable]),
            runfiles = old_default_info.default_runfiles,
            executable = new_executable,
        )
    else:
        # There's no way to determine if the underlying `go_binary` target is executable at loading time
        # so we must set the `go_cross` rule to be always executable. If the `go_binary` target is not
        # executable, we set the `go_cross` executable to a simple script that prints an error message
        # when executed. This way users can still run a `go_cross` target using `bazel run` as long as
        # the underlying `go_binary` target is executable.
        error_script = _error_script(ctx)

        # See the implementation of `go_binary` for an explanation of the need for default vs data runfiles here.
        new_default_info = DefaultInfo(
            files = depset([error_script] + old_default_info.files.to_list()),
            default_runfiles = old_default_info.default_runfiles,
            data_runfiles = old_default_info.data_runfiles.merge(ctx.runfiles([error_script])),
            executable = error_script,
        )

    providers = [
        ctx.attr.target[provider]
        for provider in [
            GoArchive,
            OutputGroupInfo,
            CcInfo,
        ]
        if provider in ctx.attr.target
    ]
    return [new_default_info] + providers

_go_cross_kwargs = {
    "implementation": _go_cross_impl,
    "attrs": {
        "target": attr.label(
            doc = """Go binary target to transition to the given platform and/or sdk_version.
            """,
            providers = [GoArchive],
            mandatory = True,
        ),
        "platform": attr.label(
            doc = """The platform to cross compile the `target` for.
            If unspecified, the `target` will be compiled with the
            same platform as it would've with the original `go_binary` rule.
            """,
        ),
        "sdk_version": attr.string(
            doc = """The golang SDK version to use for compiling the `target`.
            Supports specifying major, minor, and/or patch versions, eg. `"1"`,
            `"1.17"`, or `"1.17.1"`. The first Go SDK provider installed in the
            repo's workspace (via `go_download_sdk`, `go_wrap_sdk`, etc) that
            matches the specified version will be used for compiling the given
            `target`. If unspecified, the `target` will be compiled with the same
            SDK as it would've with the original `go_binary` rule.
            Transitions `target` by changing the `--@io_bazel_rules_go//go/toolchain:sdk_version`
            build flag to the value provided for `sdk_version` here.
            """,
        ),
        "compilation_mode": attr.string(
            doc = """The compilation_mode to use for compiling the `target`.
            Must be one of `dbg`, `fastbuild`, or `opt`. If unspecified, use the
            same compilation mode as the original `go_binary` rule.
            """,
            values = [
                "",
                "dbg",
                "fastbuild",
                "opt",
            ],
            default = "",
        ),
        "_allowlist_function_transition": attr.label(
            default = "@bazel_tools//tools/allowlists/function_transition_allowlist",
        ),
    },
    "cfg": go_cross_transition,
    "doc": """This wraps an executable built by `go_binary` to cross compile it
    for a different platform, and/or compile it using a different version
    of the golang SDK.

    **Providers:**
    <ul>
      <li>[GoArchive]</li>
    </ul>
    """,
}

go_cross_binary = rule(executable = True, **_go_cross_kwargs)
