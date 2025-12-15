// Copyright 2025 The Bazel Authors. All rights reserved.
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

package reassign_flag_commandline_test

import (
	"testing"

	"github.com/bazelbuild/rules_go/go/tools/bazel_testing"
)

// This test verifies that a test that reassigns flag.CommandLine
// and does not restore the original value after the test finishes
// does not cause 'bazel coverage' to panic.

func TestMain(m *testing.M) {
	bazel_testing.TestMain(m, bazel_testing.Args{
		Main: `
-- BUILD.bazel --
load("@io_bazel_rules_go//go:def.bzl", "go_test")

go_test(
    name = "go_default_test",
    srcs = ["test.go"],
)
-- test.go --
package main

import (
	"flag"
	"testing"
)

func TestReassign(t *testing.T) {
	flag.CommandLine = flag.NewFlagSet("test", flag.ExitOnError)
}
`,
	})
}

func Test(t *testing.T) {
	if err := bazel_testing.RunBazel("coverage", "//:go_default_test"); err != nil {
		t.Fatal(err)
	}
}
