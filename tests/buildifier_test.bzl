# Copyright 2020 The Bazel Authors. All rights reserved.
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

"""buildifier_tests.bzl provides the buildifier_test rule"""

load("@bazel_skylib//lib:shell.bzl", "shell")

def _check_file(f):
    base = f.basename
    return base in ("WORKSPACE", "BUILD", "BUILD.bazel") or base.endswith(".bzl")

def _buildifier_test_impl(ctx):
    files = [f for f in ctx.files.data if _check_file(f)]

    script = ctx.actions.declare_file(ctx.label.name + ".bash")
    content = """#!/usr/bin/env bash

set -uo pipefail

files=(
    {files}
)
buildifier={buildifier}

# warnings is the default list of warnings with exclusions:
#   bzl-visibility: we reference symbols in //go/private outside of //go.
#   confusing-name: a good font makes these very clear.
#   function-docstring: too verbose. Many functions don't need docs.
#   function-docstring-header: too verbose for now.
#   function-docstring-args: too verbose.
#   function-docstring-return: too verbose.
#   module-docstring: doesn't seem useful for many private modules.
#   name-conventions: we have non-compliant providers. We might change them
#       eventually, but we'll need to keep the old symbols for compatibility.
#   print: used for warnings.
# The list also excludes warnings buildifier 8 doesn't know: load-on-top,
# native-cc, native-java and same-origin-load.
warnings=attr-cfg,attr-license,attr-non-empty,attr-output-default,attr-single-file,build-args-kwargs,constant-glob,ctx-actions,ctx-args,depset-iteration,depset-union,dict-concatenation,duplicated-name,filetype,git-repository,http-archive,integer-division,keyword-positional-params,load,native-android,native-build,native-package,native-proto,native-py,no-effect,output-group,overly-nested-depset,package-name,package-on-top,positional-args,redefined-variable,repository-name,return-value,rule-impl-return,string-iteration,uninitialized,unreachable,unused-variable

ok=0
for file in "${{files[@]}}"; do
    "$buildifier" -mode=check -lint=warn -warnings="$warnings" "$file"
    if [ $? -ne 0 ]; then
        ok=1
    fi
done
exit $ok
""".format(
        buildifier = shell.quote(ctx.executable._buildifier.short_path),
        files = "\n".join([shell.quote(f.path) for f in files]),
    )
    ctx.actions.write(script, content, is_executable = True)

    return [DefaultInfo(
        executable = script,
        # The prebuilt buildifier is a wrapper that needs its own runfiles.
        default_runfiles = ctx.runfiles(
            files = [script] + files,
        ).merge(ctx.attr._buildifier[DefaultInfo].default_runfiles),
    )]

buildifier_test = rule(
    implementation = _buildifier_test_impl,
    attrs = {
        "data": attr.label_list(
            allow_files = True,
        ),
        "_buildifier": attr.label(
            default = "@buildifier_prebuilt//:buildifier",
            executable = True,
            cfg = "exec",
        ),
    },
    test = True,
)
