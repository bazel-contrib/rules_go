# Copyright 2018 The Bazel Authors. All rights reserved.
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

DEFAULT_NOGO = "@io_bazel_rules_go//:default_nogo"
NOGO_DEFAULT_INCLUDES = ["@@//:__subpackages__"]
NOGO_DEFAULT_EXCLUDES = []

# repr(Label(...)) does not emit a canonical label literal.
def _label_repr(label):
    return "Label(\"{}\")".format(label)

def _scope_list_repr(scopes):
    if scopes == ["all"]:
        return repr(["all"])
    return "[" + ", ".join([_label_repr(Label(l)) for l in scopes]) + "]"

def _go_register_nogo_impl(ctx):
    ctx.template(
        "BUILD.bazel",
        Label("//go/private:BUILD.nogo.bazel"),
        substitutions = {
            "{{nogo}}": ctx.attr.nogo,
        },
        executable = False,
    )
    if ctx.attr.includes:
        print("go_register_nogo's include attribute is no-op. Nogo now collect facts from all targets by default. To include files in nogo validation, please use only_files in the JSON: https://github.com/bazel-contrib/rules_go/blob/master/go/nogo.rst#example")

    ctx.file(
        "scope.bzl",
        """
EXCLUDES = {excludes}
""".format(
            excludes = _scope_list_repr(ctx.attr.excludes),
        ),
        executable = False,
    )

# go_register_nogo creates a repository with an alias that points
# to the nogo rule that should be used globally by go rules in the workspace.
# This may be called automatically by go_rules_dependencies or by
# go_register_toolchains.
# With Bzlmod, it is created by the go_sdk extension.
go_register_nogo = repository_rule(
    _go_register_nogo_impl,
    attrs = {
        "nogo": attr.string(mandatory = True),
        "includes": attr.string_list(default = None),
        "excludes": attr.string_list(),
    },
)

def go_register_nogo_wrapper(nogo, includes = None, excludes = NOGO_DEFAULT_EXCLUDES):
    """See go/nogo.rst"""
    go_register_nogo(
        name = "io_bazel_rules_nogo",
        nogo = nogo,
        includes = includes,
        excludes = excludes,
    )
