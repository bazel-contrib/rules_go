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
    """Runs a Go integration test in a private Bazel workspace.

    Adds the bazel_testing library and rule files automatically. Each test gets
    a unique workspace and output root beneath TEST_TMPDIR, with private home,
    temporary, and cache directories for nested commands. System and fixture
    workspace rc files remain enabled; home rc files are disabled. System rc
    support preserves required host settings, such as macOS JVM networking.

    The native Bazel executable selected by the enclosing build is an internal
    test dependency. Callers need neither a binary attribute nor a Bazel
    installation on PATH at test runtime. For example:

        go_bazel_test(
            name = "example_test",
            srcs = ["example_test.go"],
        )

    //tests:bazel_binary captures and validates the executable during repository
    setup, then copies its bytes into a declared input. //tests:bazel_version
    records its version. Both Bzlmod and WORKSPACE register this lazily, so
    ordinary builds do not fetch it. No Bazelisk launcher or Bazel download is
    needed while the test runs.

    LTS jobs use their enclosing Bazel version. An absolute USE_BAZEL_VERSION
    preserves the exact executable under test. To supply a native executable
    explicitly, set --repo_env=GO_BAZEL_TEST_BINARY=/absolute/native/bazel; its
    version must match the enclosing Bazel. There is no fixed-version fallback.

    Private state does not make every fixture hermetic: nested repository rules
    may fetch dependencies, PATH is inherited, and cgo tests need a compatible
    C/C++ toolchain. Remote execution also requires matching client and worker
    OS and CPU for the captured Bazel executable and host-compatible Go SDK.
    Cross-platform execution needs separate worker-compatible Bazel and SDK
    selection.

    Args:
        rule_files: Labels providing the rule files copied into the fixture.
            Defaults to //:all_files.
        **kwargs: Arguments forwarded to go_test. size defaults to "large" for
            the nested JVM and cold dependency setup; explicit sizes are kept.
    """

    bazel_binary = Label("//tests:bazel_binary")

    if not rule_files:
        rule_files = [Label("//:all_files")]

    # Add dependency on bazel_testing library.
    kwargs.setdefault("deps", [])

    bazel_testing_library = Label("//go/tools/bazel_testing")
    if bazel_testing_library not in [native.package_relative_label(dep) for dep in kwargs["deps"]]:
        kwargs["deps"].append(bazel_testing_library)

    # Add data dependency on rules_go files. bazel_testing will copy or link
    # these files in an external repo.
    kwargs.setdefault("data", [])
    kwargs["data"] += rule_files

    # Add paths to rules_go files to arguments. bazel_testing will copy or link
    # these files.
    kwargs.setdefault("args", [])
    kwargs["args"] = (["-begin_files"] +
                      ["$(rlocationpaths {})".format(t) for t in rule_files] +
                      ["-end_files"] +
                      kwargs["args"])

    kwargs["data"].append(bazel_binary)
    kwargs["args"].append("-bazel_binary=$(rlocationpath {})".format(bazel_binary))

    # Set rundir to the workspace root directory to ensure relative paths
    # are interpreted correctly.
    kwargs.setdefault("rundir", ".")

    # Each test starts a Bazel server and resolves/builds cold dependencies.
    # Account for its JVM memory and setup time, unless the caller sizes it.
    kwargs.setdefault("size", "large")

    # Nested repository rules fetch dependencies even with private test state.
    kwargs["tags"] = kwargs.get("tags", []) + ["requires-network"]

    go_test(**kwargs)
