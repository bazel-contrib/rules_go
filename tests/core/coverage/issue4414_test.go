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
-- issue.go --
package internal

func OK() {
	// This is a no-op function
}

-- issue_test.go --
package internal

import (
	"testing"
)

func TestOK(t *testing.T) {
	OK()
}

-- BUILD.bazel --
load("@io_bazel_rules_go//go:def.bzl", "go_library", "go_test")
go_library(
    name = "issue",
    srcs = ["issue.go"],
    importpath = "github.com/bakjos/test/internal",
)

go_test(
    name = "issue_test",
    srcs = ["issue_test.go"],
    embed = [":issue"],
)
`,
	})
}

func TestIssue4414(t *testing.T) {
	if err := bazel_testing.RunBazel("coverage", "--instrument_test_targets", "//:issue_test"); err != nil {
		t.Fatal(err)
	}
}
