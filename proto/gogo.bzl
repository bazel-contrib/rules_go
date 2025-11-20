# Copyright 2017 The Bazel Authors. All rights reserved.
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

def _gogo_special_proto_impl(ctx):
    ctx.file("WORKSPACE", 'workspace(name = "{}")'.format(ctx.name))
    ctx.file("BUILD.bazel", "")
    ctx.symlink(
        ctx.path(Label("@com_github_gogo_protobuf//gogoproto:gogo.proto")),
        "github.com/gogo/protobuf/gogoproto/gogo.proto",
    )
    ctx.file("github.com/gogo/protobuf/gogoproto/BUILD.bazel", """
load("@com_google_protobuf//bazel:proto_library.bzl", "proto_library")

proto_library(
    name = "gogoproto",
    srcs = [":gogo.proto"],
    tags = ["manual"],
    visibility = ["//visibility:public"],
    deps = [
        "@com_google_protobuf//:descriptor_proto",
    ],
)
  """)

gogo_special_proto = repository_rule(
    _gogo_special_proto_impl,
    attrs = {
        "proto": attr.label(allow_single_file = True),
    },
)
