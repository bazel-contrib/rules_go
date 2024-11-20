// Copyright 2019 The Bazel Authors. All rights reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//    http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package proto_test

import (
	"testing"

	"github.com/bazelbuild/rules_go/go/tools/bazel_testing"
)

var testArgs = bazel_testing.Args{
	ModuleFileSuffix: `
bazel_dep(name = "rules_proto", version = "6.0.0")
bazel_dep(name = "toolchains_protoc", version = "0.3.4")
`,
	WorkspacePrefix: `
load("@bazel_tools//tools/build_defs/repo:http.bzl", "http_archive")

# The non-polyfill version of this is needed by rules_proto below.
http_archive(
    name = "bazel_features",
    sha256 = "d7787da289a7fb497352211ad200ec9f698822a9e0757a4976fd9f713ff372b3",
    strip_prefix = "bazel_features-1.9.1",
    url = "https://github.com/bazel-contrib/bazel_features/releases/download/v1.9.1/bazel_features-v1.9.1.tar.gz",
)

load("@bazel_features//:deps.bzl", "bazel_features_deps")

bazel_features_deps()
`,
	WorkspaceSuffix: `
load("@bazel_tools//tools/build_defs/repo:http.bzl", "http_archive")

http_archive(
    name = "com_google_protobuf",
    sha256 = "75be42bd736f4df6d702a0e4e4d30de9ee40eac024c4b845d17ae4cc831fe4ae",
    strip_prefix = "protobuf-21.7",
    # latest available in BCR, as of 2022-09-30
    urls = [
        "https://github.com/protocolbuffers/protobuf/archive/v21.7.tar.gz",
        "https://mirror.bazel.build/github.com/protocolbuffers/protobuf/archive/v21.7.tar.gz",
    ],
)

load("@com_google_protobuf//:protobuf_deps.bzl", "protobuf_deps")

protobuf_deps()

http_archive(
    name = "rules_proto",
    sha256 = "303e86e722a520f6f326a50b41cfc16b98fe6d1955ce46642a5b7a67c11c0f5d",
    strip_prefix = "rules_proto-6.0.0",
    url = "https://github.com/bazelbuild/rules_proto/releases/download/6.0.0/rules_proto-6.0.0.tar.gz",
)

load("@rules_proto//proto:repositories.bzl", "rules_proto_dependencies")
rules_proto_dependencies()

load("@rules_proto//proto:toolchains.bzl", "rules_proto_toolchains")
rules_proto_toolchains()
`,
	Main: `
-- BUILD.bazel --
load("@rules_proto//proto:defs.bzl", "proto_library")
load("@io_bazel_rules_go//proto:def.bzl", "go_proto_library")
load("@io_bazel_rules_go//go:def.bzl", "go_binary")

proto_library(
    name = "cross_proto",
    srcs = ["cross.proto"],
)

go_proto_library(
    name = "cross_go_proto",
    importpath = "github.com/bazelbuild/rules_go/tests/core/cross",
    protos = [":cross_proto"],
)

go_binary(
    name = "use_bin",
    srcs = ["use.go"],
    deps = [":cross_go_proto"],
    goos = "linux",
    goarch = "386",
)

go_binary(
    name = "use_shared",
    srcs = ["use.go"],
    deps = [":cross_go_proto"],
    linkmode = "c-shared",
)

-- cross.proto --
syntax = "proto3";

package cross;

option go_package = "github.com/bazelbuild/rules_go/tests/core/cross";

message Foo {
  int64 x = 1;
}

-- use.go --
package main

import _ "github.com/bazelbuild/rules_go/tests/core/cross"

func main() {}
`,
}

func TestMain(m *testing.M) {
	bazel_testing.TestMain(m, testArgs)
}

func TestCmdLine(t *testing.T) {
	args := []string{
		"build",
		"--platforms=@io_bazel_rules_go//go/toolchain:linux_386",
		":cross_go_proto",
	}
	if err := bazel_testing.RunBazel(args...); err != nil {
		t.Fatal(err)
	}
}

func TestTargets(t *testing.T) {
	for _, target := range []string{"//:use_bin", "//:use_shared"} {
		if err := bazel_testing.RunBazel("build", target); err != nil {
			t.Errorf("building target %s: %v", target, err)
		}
	}
}
