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

load("@io_bazel_rules_go_bazel_features//:features.bzl", "bazel_features")
load("//go/private:common.bzl", "GO_TOOLCHAIN_LABEL")

def baseline_coverage_kwargs(go, ctx, sources):
    """Emits a baseline coverage report and returns it as instrumented_files_info kwargs.

    Bazel asks a non-test rule for a "baseline" coverage report so that code no
    test ever links still appears in the combined coverage report. Absent one,
    Bazel substitutes a stub that names each source file and nothing else, which
    its LcovPrinter renders as "LF:0/LH:0". A consumer cannot tell that apart
    from a file with genuinely nothing to cover, so a wholly untested package
    reports 100% rather than 0%.
    See https://github.com/bazelbuild/bazel/issues/5716.

    Returns empty kwargs, leaving Bazel's stub in place, when the running Bazel
    is too old to accept a baseline, when the build is not collecting coverage,
    or when --instrumentation_filter excludes this target. Nothing is emitted on
    an ordinary build.

    Args:
        go: the Go context.
        ctx: the rule context.
        sources: the rule's `srcs`, matching what is declared to
            coverage_common.instrumented_files_info as source attributes.

    Returns:
        A dict to splat into coverage_common.instrumented_files_info.
    """
    if not bazel_features.rules.instrumented_files_info_has_baseline_coverage_files:
        return {}
    if not go.coverage_enabled or not go.coverage_instrumented:
        return {}

    out_lcov = go.declare_file(go, name = ctx.label.name, ext = ".baseline.lcov")
    _emit_baseline_coverage(go, sources = sources, out_lcov = out_lcov)
    return {"baseline_coverage_files": [out_lcov]}

def _emit_baseline_coverage(go, *, sources, out_lcov):
    """Emits the action that writes every coverable line in sources at count zero.

    Args:
        go: the Go context.
        sources: candidate source files. Non-Go files are ignored, as are Go
            files excluded by build constraints.
        out_lcov: the File to write the LCOV tracefile to.
    """
    sdk = go.sdk
    go_srcs = [src for src in sources if src.extension == "go"]

    args = go.builder_args(go)
    args.add_all(go_srcs, before_each = "-src")
    args.add("-o", out_lcov)

    go.actions.run(
        inputs = depset(go_srcs, transitive = [sdk.headers, sdk.tools]),
        outputs = [out_lcov],
        mnemonic = "GoBaselineCoverage",
        executable = go.toolchain._builder,
        arguments = ["baselinecoverage", args],
        toolchain = GO_TOOLCHAIN_LABEL,
    )
