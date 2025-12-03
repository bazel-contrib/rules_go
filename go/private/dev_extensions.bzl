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

load("@io_bazel_rules_go_bazel_features//:features.bzl", "bazel_features")
load("@gazelle//:deps.bzl", "go_repository")

def _dev_deps_impl(ctx):
    go_repository(
        name = "org_golang_google_genproto",
        build_extra_args = ["-exclude=vendor"],
        build_file_generation = "on",
        build_file_proto_mode = "disable_global",
        importpath = "google.golang.org/genproto",
        sum = "h1:S9GbmC1iCgvbLyAokVCwiO6tVIrU9Y7c5oMx1V/ki/Y=",
        version = "v0.0.0-20221024183307-1bc688fe9f3e",
    )

    if bazel_features.external_deps.extension_metadata_has_reproducible:
        kwargs = {
            "reproducible": True,
        }

        return ctx.extension_metadata(**kwargs)
    else:
        return None

dev_deps = module_extension(
    implementation = _dev_deps_impl,
)
