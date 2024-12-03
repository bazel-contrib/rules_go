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

package lcov_coverage_test

import (
	"io/ioutil"
	"path/filepath"
	"strings"
	"testing"

	"github.com/bazelbuild/rules_go/go/tools/bazel_testing"
)

func TestMain(m *testing.M) {
	bazel_testing.TestMain(m, bazel_testing.Args{
		Main: `
-- src/BUILD.bazel --
load("@io_bazel_rules_go//go:def.bzl", "go_library", "go_test")

go_library(
    name = "lib",
    srcs = ["lib.go"],
    importpath = "example.com/lib",
    deps = [":other_lib"],
)

go_library(
    name = "other_lib",
    srcs = ["other_lib.go"],
    importpath = "example.com/other_lib",
)

go_test(
    name = "lib_test",
    srcs = ["lib_test.go"],
    deps = [":lib"],
)

java_binary(
    name = "Tool",
    srcs = ["Tool.java"],
)

go_test(
    name = "lib_with_tool_test",
    srcs = ["lib_with_tool_test.go"],
    data = [":Tool"],
    deps = [":lib"],
)
-- src/lib.go --
package lib

import (
	"strings"

	"example.com/other_lib"
)

func HelloFromLib(informal bool) string {
	var greetings []string
	if informal {
		greetings = []string{"Hey there, other_lib!"}
	} else {
		greetings = []string{"Good morning, other_lib!"}
	}
	greetings = append(greetings, other_lib.HelloOtherLib(informal))
	return strings.Join(greetings, "\n")
}
-- src/other_lib.go --
package other_lib

func HelloOtherLib(informal bool) string {
	if informal {
		return "Hey there, other_lib!"
	}
	return "Good morning, other_lib!"
}
-- src/lib_test.go --
package lib_test

import (
	"strings"
	"testing"

	"example.com/lib"
)

func TestLib(t *testing.T) {
	if !strings.Contains(lib.HelloFromLib(false), "\n") {
		t.Error("Expected a newline in the output")
	}
}
-- src/Tool.java --
public class Tool {
  public static void main(String[] args) {
    if (args.length != 0) {
      System.err.println("Expected no arguments");
      System.exit(1);
    }
    System.err.println("Hello, world!");
  }
}
-- src/lib_with_tool_test.go --
package lib_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"example.com/lib"
)

func TestLib(t *testing.T) {
	// Test that coverage is collected even if this variable is corrupted by a test.
	os.Setenv("COVERAGE_DIR", "invalid")
	if !strings.Contains(lib.HelloFromLib(false), "\n") {
		t.Error("Expected a newline in the output")
	}
}

func TestTool(t *testing.T) {
	err := exec.Command("." + string(filepath.Separator) + "Tool").Run()
	if err != nil {
		t.Error(err)
	}
}

`,
	})
}

func TestLcovCoverage(t *testing.T) {
	t.Run("without-race", func(t *testing.T) {
		testLcovCoverage(t)
	})

	t.Run("with-race", func(t *testing.T) {
		testLcovCoverage(t, "--@io_bazel_rules_go//go/config:race")
	})
}

func testLcovCoverage(t *testing.T, extraArgs ...string) {
	args := append([]string{
		"coverage",
		"--combined_report=lcov",
		"//src:lib_test",
	}, extraArgs...)

	if err := bazel_testing.RunBazel(args...); err != nil {
		t.Fatal(err)
	}

	individualCoveragePath := filepath.FromSlash("bazel-testlogs/src/lib_test/coverage.dat")
	individualCoverageData, err := ioutil.ReadFile(individualCoveragePath)
	if err != nil {
		t.Fatal(err)
	}
	for _, expectedIndividualCoverage := range expectedGoCoverage {
		if !strings.Contains(string(individualCoverageData), expectedIndividualCoverage) {
			t.Errorf(
				"%s: does not contain:\n\n%s\nactual content:\n\n%s",
				individualCoveragePath,
				expectedIndividualCoverage,
				string(individualCoverageData),
			)
		}
	}

	combinedCoveragePath := filepath.FromSlash("bazel-out/_coverage/_coverage_report.dat")
	combinedCoverageData, err := ioutil.ReadFile(combinedCoveragePath)
	if err != nil {
		t.Fatal(err)
	}
	for _, include := range []string{
		"SF:src/lib.go\n",
		"SF:src/other_lib.go\n",
	} {
		if !strings.Contains(string(combinedCoverageData), include) {
			t.Errorf("%s: does not contain %q\n", combinedCoverageData, include)
		}
	}
}

func TestLcovCoverageWithTool(t *testing.T) {
	args := append([]string{
		"coverage",
		"--combined_report=lcov",
		"--java_runtime_version=remotejdk_11",
		"//src:lib_with_tool_test",
	})

	if err := bazel_testing.RunBazel(args...); err != nil {
		t.Fatal(err)
	}

	individualCoveragePath := filepath.FromSlash("bazel-testlogs/src/lib_with_tool_test/coverage.dat")
	individualCoverageData, err := ioutil.ReadFile(individualCoveragePath)
	if err != nil {
		t.Fatal(err)
	}
	expectedCoverage := append(expectedGoCoverage, expectedToolCoverage)
	for _, expected := range expectedCoverage {
		if !strings.Contains(string(individualCoverageData), expected) {
			t.Errorf(
				"%s: does not contain:\n\n%s\nactual content:\n\n%s",
				individualCoveragePath,
				expected,
				string(individualCoverageData),
			)
		}
	}

	combinedCoveragePath := filepath.FromSlash("bazel-out/_coverage/_coverage_report.dat")
	combinedCoverageData, err := ioutil.ReadFile(combinedCoveragePath)
	if err != nil {
		t.Fatal(err)
	}
	for _, include := range []string{
		"SF:src/lib.go\n",
		"SF:src/other_lib.go\n",
		"SF:src/Tool.java\n",
	} {
		if !strings.Contains(string(combinedCoverageData), include) {
			t.Errorf("%s: does not contain %q\n", combinedCoverageData, include)
		}
	}
}

var expectedGoCoverage = []string{
	`SF:src/other_lib.go
FNF:0
FNH:0
DA:3,1
DA:4,1
DA:5,0
DA:6,0
DA:7,1
LH:3
LF:5
end_of_record
`,
	`SF:src/lib.go
FNF:0
FNH:0
DA:9,1
DA:10,1
DA:11,1
DA:12,0
DA:13,1
DA:14,1
DA:15,1
DA:16,1
DA:17,1
LH:8
LF:9
end_of_record
`}

const expectedToolCoverage = `SF:src/Tool.java
FN:1,Tool::<init> ()V
FN:3,Tool::main ([Ljava/lang/String;)V
FNDA:0,Tool::<init> ()V
FNDA:1,Tool::main ([Ljava/lang/String;)V
FNF:2
FNH:1
BRDA:3,0,0,1
BRDA:3,0,1,0
BRF:2
BRH:1
DA:1,0
DA:3,1
DA:4,0
DA:5,0
DA:7,1
DA:8,1
LH:3
LF:6
end_of_record
`
