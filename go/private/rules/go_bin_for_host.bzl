# Copyright 2023 The Bazel Authors. All rights reserved.
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

load("@platforms//host:constraints.bzl", "HOST_CONSTRAINTS")
load("//go/private:common.bzl", "GO_TOOLCHAIN")

def _ensure_target_cfg(ctx):
    # A target is assumed to be built in the target configuration if it is neither in the exec nor
    # the host configuration (the latter has been removed in Bazel 6). Since there is no API for
    # this, use the output directory to determine the configuration, which is a common pattern.
    if "-exec" in ctx.bin_dir.path or "/host/" in ctx.bin_dir.path:
        fail("//go is only meant to be used with 'bazel run', not as a tool. " +
             "If you need to use it as a tool (e.g. in a genrule), please " +
             "open an issue at " +
             "https://github.com/bazelbuild/rules_go/issues/new explaining " +
             "your use case.")

def _go_bin_for_host_impl(ctx):
    """Exposes the go binary of the current Go toolchain for the host."""
    _ensure_target_cfg(ctx)

    sdk = ctx.toolchains[GO_TOOLCHAIN].sdk
    sdk_files = ctx.runfiles(
        [sdk.go],
        transitive_files = depset(transitive = [sdk.headers, sdk.srcs, sdk.libs, sdk.tools]),
    )

    return [
        DefaultInfo(
            files = depset([sdk.go]),
            runfiles = sdk_files,
        ),
    ]

go_bin_for_host = rule(
    implementation = _go_bin_for_host_impl,
    toolchains = [GO_TOOLCHAIN],
    # Resolve a toolchain that runs on the host platform.
    exec_compatible_with = HOST_CONSTRAINTS,
)
