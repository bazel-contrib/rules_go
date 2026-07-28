// Copyright 2026 The Bazel Authors. All rights reserved.
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

package baseline_coverage_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/bazelbuild/rules_go/go/tools/bazel_testing"
)

func TestMain(m *testing.M) {
	bazel_testing.TestMain(m, bazel_testing.Args{
		Main: `
-- src/BUILD.bazel --
load("@io_bazel_rules_go//go:def.bzl", "go_binary", "go_library", "go_test")

# Linked by a test, so its coverage is measured for real. Present here to prove
# the baseline never displaces measured data.
go_library(
    name = "tested",
    srcs = ["tested.go"],
    importpath = "example.com/tested",
)

go_test(
    name = "tested_test",
    srcs = ["tested_test.go"],
    embed = [":tested"],
)

# No test anywhere links this. Before baseline coverage it appeared in the
# combined report as an empty record, which reads as 100%.
go_library(
    name = "untested",
    srcs = ["untested.go"],
    importpath = "example.com/untested",
)

# Declarations only: there is genuinely nothing to cover, which must stay
# distinguishable from the case above.
go_library(
    name = "declarations",
    srcs = ["declarations.go"],
    importpath = "example.com/declarations",
)

# Excluded by a build constraint that never matches, so it must not be reported
# as uncovered.
go_library(
    name = "constrained",
    srcs = ["constrained.go"],
    importpath = "example.com/constrained",
)

go_binary(
    name = "untested_bin",
    srcs = ["untested_bin.go"],
)
-- src/tested.go --
package tested

func Greet(informal bool) string {
	if informal {
		return "hi"
	}
	return "hello"
}
-- src/tested_test.go --
package tested

import "testing"

func TestGreet(t *testing.T) {
	if Greet(false) != "hello" {
		t.Error("expected hello")
	}
}
-- src/untested.go --
package untested

func Sum(xs []int) int {
	total := 0
	for _, x := range xs {
		total += x
	}
	return total
}
-- src/declarations.go --
package declarations

type Config struct {
	Name string
}

const Answer = 42
-- src/constrained.go --
//go:build ignore_me

package constrained

func Unreachable() string {
	return "never compiled"
}
-- src/untested_bin.go --
package main

import "fmt"

func main() {
	fmt.Println("nobody tests me")
}
`,
		// Baseline coverage needs Bazel 9, which dropped WORKSPACE support, so
		// this test's generated workspace must use bzlmod. A non-empty suffix is
		// what opts it in; no extra dependencies are required.
		ModuleFileSuffix: `
# Intentionally empty.
`,
	})
}

// record returns the lines of the LCOV record for path, or nil when the report
// has no record for it.
func record(t *testing.T, report, path string) []string {
	t.Helper()

	var out []string
	inRecord := false
	for _, line := range strings.Split(report, "\n") {
		switch {
		case line == "SF:"+path:
			inRecord = true
			out = nil
		case inRecord && line == "end_of_record":
			return out
		case inRecord:
			out = append(out, line)
		}
	}
	return nil
}

func has(lines []string, want string) bool {
	for _, line := range lines {
		if line == want {
			return true
		}
	}
	return false
}

func TestBaselineCoverage(t *testing.T) {
	if err := bazel_testing.RunBazel(
		"coverage",
		"--combined_report=lcov",
		"//src:all",
	); err != nil {
		t.Fatal(err)
	}

	raw, err := os.ReadFile(filepath.FromSlash("bazel-out/_coverage/_coverage_report.dat"))
	if err != nil {
		t.Fatal(err)
	}
	report := string(raw)

	t.Run("untested library reports its coverable lines as missed", func(t *testing.T) {
		lines := record(t, report, "src/untested.go")
		if lines == nil {
			t.Fatal("expected a record for src/untested.go, found none")
		}
		// Every line of Sum's body, none of them executed.
		for _, want := range []string{"DA:3,0", "DA:4,0", "DA:5,0", "DA:6,0", "DA:8,0", "LH:0", "LF:6"} {
			if !has(lines, want) {
				t.Errorf("expected %q in the record for src/untested.go: %v", want, lines)
			}
		}
	})

	t.Run("untested binary reports its coverable lines as missed", func(t *testing.T) {
		lines := record(t, report, "src/untested_bin.go")
		if lines == nil {
			t.Fatal("expected a record for src/untested_bin.go, found none")
		}
		if !has(lines, "LH:0") || has(lines, "LF:0") {
			t.Errorf("expected src/untested_bin.go to report missed lines: %v", lines)
		}
	})

	t.Run("declaration-only file has nothing to cover", func(t *testing.T) {
		lines := record(t, report, "src/declarations.go")
		if lines == nil {
			t.Fatal("expected a record for src/declarations.go, found none")
		}
		if !has(lines, "LF:0") {
			t.Errorf("expected LF:0 for src/declarations.go: %v", lines)
		}
	})

	t.Run("build-constrained file is not reported", func(t *testing.T) {
		if lines := record(t, report, "src/constrained.go"); lines != nil {
			t.Errorf("expected no record for src/constrained.go, got %v", lines)
		}
	})

	t.Run("measured coverage is not displaced by the baseline", func(t *testing.T) {
		lines := record(t, report, "src/tested.go")
		if lines == nil {
			t.Fatal("expected a record for src/tested.go, found none")
		}
		if !has(lines, "DA:3,1") {
			t.Errorf("expected the measured count DA:3,1 for src/tested.go: %v", lines)
		}
		if has(lines, "LH:0") {
			t.Errorf("expected src/tested.go to report hits: %v", lines)
		}
	})
}
