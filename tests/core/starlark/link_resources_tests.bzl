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

"""Check lld thread parsing and resource estimates across Bazel versions."""

load("@bazel_skylib//lib:unittest.bzl", "asserts", "unittest")
load("//go/private/actions:link_resources.bzl", "golink_resource_set", "parse_lld_thread_count")

def _link_resources_test_impl(ctx):
    env = unittest.begin(ctx)
    for flags, expected in [
        ([], 1),
        (["-lm"], 1),
        (["-Wl,--threads=3"], 3),
        (["--threads=8"], 8),
        (["--threads", "5"], 5),
        (["--threads  \t 6"], 6),
        (["-Wl,--threads,6"], 6),
        (["-Xlinker", "--threads", "-Xlinker", "7"], 7),
        (["-fuse-ld=lld -Wl,--threads=4 -lm"], 4),
        (["--threads=2", "-Wl,--threads=5"], 5),
        (["--threads=8", "--threads", "3"], 3),
        (["--threads=0"], 1),
        (["--threads=-2"], 1),
        (["--threads=invalid"], 1),
    ]:
        asserts.equals(env, expected, parse_lld_thread_count(flags))

    for threads in range(1, 9):
        resources = golink_resource_set(["-Wl,--threads={}".format(threads)], ctx.attr.bazel_version)
        asserts.equals(env, ctx.attr.resource_dict, type(resources) == "dict")
        if type(resources) != "dict":
            resources = resources("linux", 123)
        asserts.equals(env, {"cpu": threads, "memory": 512}, resources)

    for threads in [9, 16, 32, 128, 1024]:
        resources = golink_resource_set(["--threads={}".format(threads)], ctx.attr.bazel_version)
        if ctx.attr.resource_dict:
            asserts.equals(env, {"cpu": threads, "memory": 512}, resources)
        else:
            asserts.equals(env, {"cpu": 8, "memory": 512}, resources("linux", 123))

    return unittest.end(env)

_link_resources_test = unittest.make(
    _link_resources_test_impl,
    attrs = {
        "bazel_version": attr.string(),
        "resource_dict": attr.bool(),
    },
)

def link_resources_test_suite():
    for version in ["7.7.0", "8.8.2", "9.1.1", "9.3.0", "9.3.3", ""]:
        _link_resources_test(
            name = "link_resources_{}_test".format(version.replace(".", "_") or "development"),
            size = "small",
            bazel_version = version,
            resource_dict = version in ["9.3.0", "9.3.3"],
        )
