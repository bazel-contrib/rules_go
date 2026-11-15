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

package coverage_test

import (
	"testing"

	"github.com/bazelbuild/rules_go/go/tools/bazel_testing"
)

func TestMain(m *testing.M) {
	bazel_testing.TestMain(m, bazel_testing.Args{
		Main: `
-- BUILD.bazel --
load("@io_bazel_rules_go//go:def.bzl", "go_test", "go_tool_library", "nogo")

go_test(
    name = "coverage_target",
    srcs = ["coverage_target_test.go"],
    deps = [":coverage_target_dep"],
)

go_tool_library(
    name = "coverage_target_dep",
    srcs = ["coverage_target_dep.go"],
    importmap = "mapped/coverage_target/dep",
    importpath = "coverage_target/dep",
)

nogo(
    name = "nogo",
    vet = True,
    visibility = ["//visibility:public"],
)
-- coverage_target_test.go --
package coverage_target_test

import "coverage_target/dep"

var _ dep.Value = dep.Value{Text: "ok"}

-- coverage_target_dep.go --
package dep

type Value struct { Text string }

`,
		Nogo: `@//:nogo`,
	})
}

func TestCoverageWithNogo(t *testing.T) {
	if out, err := bazel_testing.BazelOutput("coverage", "//:coverage_target"); err != nil {
		println(string(out))
		t.Fatal(err)
	}
}

func TestCoverageOfNogo(t *testing.T) {
	if out, err := bazel_testing.BazelOutput("build", "--instrumentation_filter=.*", "--collect_code_coverage", "//:nogo"); err != nil {
		println(string(out))
		t.Fatal(err)
	}
}
