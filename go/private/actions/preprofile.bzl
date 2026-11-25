# Copyright 2026 The Bazel Go Rules Authors. All rights reserved.
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
    "GO_TOOLCHAIN_LABEL",
    "SUPPORTS_PATH_MAPPING_REQUIREMENT",
)
load(
    "//go/private:sdk.bzl",
    "parse_version",
)

def _dirname(file):
    return file.dirname

def emit_preprofile(ctx, toolchain, profile):
    """Converts a pprof profile into the intermediate representation used by the compiler.

    Args:
        ctx: the rule context.
        toolchain: the Go toolchain providing the SDK and the builder.
        profile: the pprof profile File to convert.

    Returns:
        The converted profile, or the profile unchanged if the SDK is too old to
        convert it.
    """
    sdk = toolchain.sdk

    # "go tool preprofile" was added in Go 1.23. Older compilers only accept raw
    # pprof profiles.
    version = parse_version(sdk.version)
    if not version or version[:2] < (1, 23):
        return profile

    out = ctx.actions.declare_file(ctx.label.name + "/" + profile.basename + ".preprofile")

    args = ctx.actions.args()
    args.add("preprofile")

    # Use a file rather than sdk.root_file.dirname as the latter is just a
    # string and thus not subject to path mapping.
    args.add_all("-sdk", [sdk.root_file], map_each = _dirname, expand_directories = False)
    args.add("-in", profile)
    args.add("-out", out)

    ctx.actions.run(
        inputs = depset([profile, sdk.root_file], transitive = [sdk.tools]),
        outputs = [out],
        mnemonic = "GoPreprofile",
        executable = toolchain._builder,
        arguments = [args],
        toolchain = GO_TOOLCHAIN_LABEL,
        execution_requirements = SUPPORTS_PATH_MAPPING_REQUIREMENT,
    )
    return out
