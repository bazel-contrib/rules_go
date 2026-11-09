// Copyright 2022 The Bazel Authors. All rights reserved.
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

package generics_test

import (
	"bytes"
	"testing"

	"github.com/bazelbuild/rules_go/go/tools/bazel_testing"
)

func TestMain(m *testing.M) {
	bazel_testing.TestMain(m, bazel_testing.Args{
		Nogo:         "@//:nogo",
		NogoIncludes: []string{"@//:__pkg__", "@//leaf:__pkg__"},
		Main: `
-- BUILD.bazel --
load("@io_bazel_rules_go//go:def.bzl", "go_library", "nogo")

nogo(
    name = "nogo",
    visibility = ["//visibility:public"],
    deps = ["@org_golang_x_tools//go/analysis/passes/buildssa"],
)

go_library(
    name = "src",
    srcs = ["src.go"],
    importpath = "src",
)

-- src.go --
package src

type Set[T comparable] struct {
	m map[T]struct{}
}

func New[T comparable](s ...T) *Set[T] {
	set := &Set[T]{}
	set.Add(s...)
	return set
}

func (set *Set[T]) Add(s ...T) {
	if set.m == nil {
		set.m = make(map[T]struct{})
	}
	for _, s := range s {
		set.m[s] = struct{}{}
	}
}

func S(x ...string) *Set[string] {
	return New[string](x...)
}

-- base/BUILD.bazel --
load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "base",
    srcs = ["base.go"],
    importmap = "example.com/base/implementation",
    importpath = "example.com/base",
    importpath_aliases = ["example.com/base-alias"],
    visibility = ["//visibility:public"],
)

-- base/base.go --
package base

type Box[T any] struct { Value T }

-- left/BUILD.bazel --
load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "left",
    srcs = ["left.go"],
    importpath = "example.com/left",
    deps = ["//base"],
    visibility = ["//visibility:public"],
)

-- left/left.go --
package left

import (
    "net/url"
    "example.com/base"
)

type Exposed = base.Box[url.URL]

-- right/BUILD.bazel --
load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "right",
    srcs = ["right.go"],
    importpath = "example.com/right",
    deps = ["//base"],
    visibility = ["//visibility:public"],
)

-- right/right.go --
package right

import (
    "net/url"
    "example.com/base-alias"
)

func New() base.Box[url.URL] { return base.Box[url.URL]{Value: url.URL{Scheme: "https"}} }

-- leaf/BUILD.bazel --
load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "leaf",
    srcs = ["leaf.go"],
    importpath = "example.com/leaf",
    deps = ["//left", "//right"],
)

-- leaf/leaf.go --
package leaf

import (
    "net/url"
    "example.com/left"
    "example.com/right"
)

func New() (left.Exposed, *url.URL) {
    value := right.New()
    return value, &value.Value
}
`,
	})
}

func Test(t *testing.T) {
	cmd := bazel_testing.BazelCmd("build", "//:src", "//leaf")
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		t.Log("output:", stderr.String())
		t.Fatal("unexpected error:", err)
	}

	if bytes.Contains(stderr.Bytes(), []byte("panic")) {
		t.Errorf("found panic in Bazel output: \n%s", stderr.String())
	}
}
