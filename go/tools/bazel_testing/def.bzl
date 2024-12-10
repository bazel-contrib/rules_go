# Copyright 2019 The Bazel Authors. All rights reserved.
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

load("//go:def.bzl", "go_test")

def go_bazel_test(rule_files = None, **kwargs):
    """go_bazel_test is a wrapper for go_test that simplifies the use of
    //go/tools/bazel_testing. Tests may be written
    that don't explicitly depend on bazel_testing or rules_go files.
    """

    if not rule_files:
        rule_files = [Label("//:all_files")]

    # Add dependency on bazel_testing library.
    kwargs.setdefault("deps", [])

    bazel_testing_library = "@io_bazel_rules_go//go/tools/bazel_testing"
    if bazel_testing_library not in kwargs["deps"]:
        kwargs["deps"].append(bazel_testing_library)

    # Add data dependency on rules_go files. bazel_testing will copy or link
    # these files in an external repo.
    kwargs.setdefault("data", [])
    kwargs["data"] += rule_files

    # Add paths to rules_go files to arguments. bazel_testing will copy or link
    # these files.
    kwargs.setdefault("args", [])
    kwargs["args"] = (["-begin_files"] +
                      ["$(locations {})".format(t) for t in rule_files] +
                      ["-end_files"] +
                      kwargs["args"])

    kwargs.setdefault("env", {})
    kwargs["env"]["BZLMOD_ENABLED"] = str(str(Label("//:foo")).startswith("@@"))

    # Set rundir to the workspace root directory to ensure relative paths
    # are interpreted correctly.
    kwargs.setdefault("rundir", ".")

    # Set tags.
    # local: don't run in sandbox or on remote executor.
    # exclusive: run one test at a time, since they share a Bazel
    #   output directory. If we don't do this, tests must extract the bazel
    #   installation and start with a fresh cache every time, making them
    #   much slower.
    kwargs.setdefault("tags", [])
    if "local" not in kwargs["tags"]:
        kwargs["tags"].append("local")
    if "exclusive" not in kwargs["tags"]:
        kwargs["tags"].append("exclusive")

    go_test(**kwargs)
