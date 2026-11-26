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

"""Expose the running Bazel version to loading and analysis code."""

def _bazel_version_repo_impl(ctx):
    ctx.file("version.bzl", "BAZEL_VERSION = {}\n".format(repr(native.bazel_version)))
    ctx.file("BUILD.bazel", """
load("@bazel_skylib//:bzl_library.bzl", "bzl_library")
exports_files(["version.bzl"])
bzl_library(
    name = "version",
    srcs = ["version.bzl"],
    visibility = ["//visibility:public"],
)
""")

bazel_version_repo = repository_rule(
    implementation = _bazel_version_repo_impl,
    local = True,
)
