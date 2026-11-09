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

"""Writes the stdlib_uses_cgo results that stdlib_cgo_predicate_test checks."""

load("//go/private:platforms.bzl", "GOOS_GOARCH")
load("//go/private/actions:link.bzl", "stdlib_uses_cgo")

# The build tag sets to check for each platform. Go reads the boringcrypto
# build constraint as goexperiment.boringcrypto, so the plain boringcrypto tag
# must not change the result, and goexperiment.boringcrypto must.
_TAG_VARIANTS = [
    [],
    ["netgo"],
    ["osusergo"],
    ["netgo", "osusergo"],
    ["netgo", "osusergo", "boringcrypto"],
    ["netgo", "osusergo", "goexperiment.boringcrypto"],
]

def _stdlib_cgo_predicate_matrix_impl(ctx):
    rows = [
        {
            "goarch": goarch,
            "goos": goos,
            "tags": tags,
            "uses_cgo": stdlib_uses_cgo(goos, goarch, tags, ""),
        }
        for goos, goarch in GOOS_GOARCH
        for tags in _TAG_VARIANTS
    ]
    out = ctx.actions.declare_file(ctx.label.name + ".json")
    ctx.actions.write(out, json.encode_indent(rows))
    return [DefaultInfo(
        files = depset([out]),
        runfiles = ctx.runfiles([out]),
    )]

stdlib_cgo_predicate_matrix = rule(
    implementation = _stdlib_cgo_predicate_matrix_impl,
    doc = "Writes stdlib_uses_cgo for each platform in platforms.bzl and each tag set as JSON.",
)
