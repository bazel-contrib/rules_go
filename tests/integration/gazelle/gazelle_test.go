// Copyright 2020 The Bazel Authors. All rights reserved.
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

package gazelle_test

import (
	"io/ioutil"
	"strings"
	"testing"

	"github.com/bazelbuild/rules_go/go/tools/bazel_testing"
)

func TestMain(m *testing.M) {
	bazel_testing.TestMain(m, bazel_testing.Args{
		Main: `
-- BUILD.bazel --
load("@bazel_gazelle//:def.bzl", "gazelle")

# gazelle:prefix example.com/hello
gazelle(
    name = "gazelle",
)
-- hello.go --
package hello
`,
		ModuleFileSuffix: `
bazel_dep(name = "gazelle", version = "0.41.0", repo_name = "bazel_gazelle")
`,
	})
}

func TestUpdate(t *testing.T) {
	if err := bazel_testing.RunBazel("run", "//:gazelle"); err != nil {
		t.Fatal(err)
	}
	data, err := ioutil.ReadFile("BUILD.bazel")
	if err != nil {
		t.Fatal(err)
	}
	got := strings.TrimSpace(string(data))
	want := strings.TrimSpace(`
load("@bazel_gazelle//:def.bzl", "gazelle")
load("@io_bazel_rules_go//go:def.bzl", "go_library")

# gazelle:prefix example.com/hello
gazelle(
    name = "gazelle",
)

go_library(
    name = "hello",
    srcs = ["hello.go"],
    importpath = "example.com/hello",
    visibility = ["//visibility:public"],
)
`)
	if got != want {
		t.Errorf("got:\n%s\n\nwant:\n%s", got, want)
	}
}
