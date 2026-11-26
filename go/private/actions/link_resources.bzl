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

"""Local resource estimates for externally linked Go binaries."""

load("@io_bazel_rules_go_bazel_version//:version.bzl", "BAZEL_VERSION")
load("//go/private/skylib/lib:versions.bzl", "versions")

def parse_lld_thread_count(extldflags):
    """Returns the last positive lld --threads setting, or one if absent."""
    threads = 1
    expect_count = False
    for flag in extldflags:
        for token in flag.replace("\t", " ").replace("\n", " ").split(" "):
            for part in token.split(","):
                if not part or part in ["-Wl", "-Xlinker"]:
                    continue
                if expect_count and part.isdigit() and int(part) > 0:
                    threads = int(part)
                expect_count = part == "--threads"
                if part.startswith("--threads="):
                    count = part.removeprefix("--threads=")
                    if count.isdigit() and int(count) > 0:
                        threads = int(count)
    return threads

# Older Bazel versions require a top-level callback and cannot bind a CPU count.
def _resources_1(_os, _inputs):
    return {"cpu": 1, "memory": 512}

def _resources_2(_os, _inputs):
    return {"cpu": 2, "memory": 512}

def _resources_3(_os, _inputs):
    return {"cpu": 3, "memory": 512}

def _resources_4(_os, _inputs):
    return {"cpu": 4, "memory": 512}

def _resources_5(_os, _inputs):
    return {"cpu": 5, "memory": 512}

def _resources_6(_os, _inputs):
    return {"cpu": 6, "memory": 512}

def _resources_7(_os, _inputs):
    return {"cpu": 7, "memory": 512}

def _resources_8(_os, _inputs):
    return {"cpu": 8, "memory": 512}

_RESOURCE_CALLBACKS = {
    1: _resources_1,
    2: _resources_2,
    3: _resources_3,
    4: _resources_4,
    5: _resources_5,
    6: _resources_6,
    7: _resources_7,
    8: _resources_8,
}

def golink_resource_set(extldflags, bazel_version = BAZEL_VERSION):
    """Returns a resource dictionary on Bazel 9.3+, or a compatible callback."""
    threads = parse_lld_thread_count(extldflags)
    if bazel_version and versions.is_at_least("9.3.0", bazel_version):
        return {"cpu": threads, "memory": 512}

    # Cap legacy resource estimates at the largest available callback.
    return _RESOURCE_CALLBACKS[min(threads, 8)]
