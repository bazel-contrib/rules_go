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
	"bytes"
	"fmt"
	"io/ioutil"
	"path/filepath"
	"testing"

	"github.com/bazelbuild/rules_go/go/tools/bazel_testing"
)

func TestMain(m *testing.M) {
	bazel_testing.TestMain(m, bazel_testing.Args{
		Main: `
-- BUILD.bazel --
load("@io_bazel_rules_go//go:def.bzl", "go_library", "go_test")

go_test(
    name = "a_test",
    srcs = ["a_test.go"],
    embed = [":a"],
)

go_test(
    name = "a_test_cross",
    srcs = ["a_test.go"],
    embed = [":a"],
    goarch = "386",
    goos = "linux",
    pure = "on",
    tags = ["manual"],
)

go_library(
    name = "a",
    srcs = ["a.go"],
    importpath = "example.com/coverage/a",
    deps = [":b"],
)

go_library(
    name = "b",
    srcs = ["b.go"],
    importpath = "example.com/coverage/b",
    deps = [":c"],
)

go_library(
    name = "c",
    srcs = ["c.go"],
    importpath = "example.com/coverage/c",
)

go_library(
	name = "d",
	srcs = ["d.go"],
	importpath = "example.com/coverage/d",
)

go_test(
	name = "d_test",
	embed = [":d"],
)

go_library(
	name = "panicking",
	srcs = ["panicking.go"],
	importpath = "example.com/coverage/panicking",
)

go_test(
    name = "panicking_test",
    srcs = ["panicking_test.go"],
    embed = [":panicking"],
)
-- a_test.go --
package a

import "testing"

func TestA(t *testing.T) {
	ALive()
}
-- a.go --
package a

import "example.com/coverage/b"

func ALive() int {
	return b.BLive()
}

func ADead() int {
	return b.BDead()
}

-- b.go --
package b

import "example.com/coverage/c"

func BLive() int {
	return c.CLive()
}

func BDead() int {
	return c.CDead()
}

-- c.go --
package c

func CLive() int {
	return 12
}

func CDead() int {
	return 34
}

-- d.go --
package lzma

/* Naming conventions follows the CodeReviewComments in the Go Wiki. */

// ntz32Const is used by the functions NTZ and NLZ.
const ntz32Const = 0x04d7651f
-- panicking.go --
package panicking

func Panic() {
	panic("from line 4")
}
-- panicking_test.go --
package panicking

import (
	"regexp"
	"runtime/debug"
	"testing"
)

func TestPanic(t *testing.T) {
	defer func() {
		if err := recover(); err != nil {
			got := regexp.MustCompile("panicking.go:[0-9]+").
				FindString(string(debug.Stack()))
			if want := "panicking.go:4"; want != got {
				t.Errorf("want %q; got %q", want, got)
			}
		}
	}()
	Panic()
}
`,
	})
}

func TestCoverage(t *testing.T) {
	t.Run("without-race", func(t *testing.T) {
		testCoverage(t, "atomic")
	})

	t.Run("with-race", func(t *testing.T) {
		testCoverage(t, "atomic", "--@io_bazel_rules_go//go/config:race")
	})
}

func testCoverage(t *testing.T, expectedCoverMode string, extraArgs ...string) {
	args := append([]string{"coverage"}, append(
		extraArgs,
		"--instrumentation_filter=-//:b",
		"--@io_bazel_rules_go//go/config:cover_format=go_cover",
		":a_test",
	)...)

	if err := bazel_testing.RunBazel(args...); err != nil {
		t.Fatal(err)
	}

	coveragePath := filepath.FromSlash("bazel-testlogs/a_test/coverage.dat")
	coverageData, err := ioutil.ReadFile(coveragePath)
	if err != nil {
		t.Fatal(err)
	}
	for _, include := range []string{
		fmt.Sprintf("mode: %s", expectedCoverMode),
		"example.com/coverage/a/a.go:",
		"example.com/coverage/c/c.go:",
	} {
		if !bytes.Contains(coverageData, []byte(include)) {
			t.Errorf("%s: does not contain %q\n", coveragePath, include)
		}
	}
	for _, exclude := range []string{
		"example.com/coverage/b/b.go:",
	} {
		if bytes.Contains(coverageData, []byte(exclude)) {
			t.Errorf("%s: contains %q\n", coveragePath, exclude)
		}
	}
}

func TestCrossBuild(t *testing.T) {
	t.Run("lcov", func(t *testing.T) {
		testCrossBuild(t)
	})
	t.Run("cover", func(t *testing.T) {
		testCrossBuild(t, "--@io_bazel_rules_go//go/config:cover_format=go_cover")
	})
}

func testCrossBuild(t *testing.T, extraArgs ...string) {
	if err := bazel_testing.RunBazel(append(
		[]string{"build", "--collect_code_coverage", "--instrumentation_filter=-//:b", "//:a_test_cross"},
		extraArgs...,
	)...); err != nil {
		t.Fatal(err)
	}
}

func TestCoverageWithComments(t *testing.T) {
	t.Run("lcov", func(t *testing.T) {
		testCoverageWithComments(t)
	})
	t.Run("go_cover", func(t *testing.T) {
		testCoverageWithComments(t, "--@io_bazel_rules_go//go/config:cover_format=go_cover")
	})
}

func testCoverageWithComments(t *testing.T, extraArgs ...string) {
	if err := bazel_testing.RunBazel(append([]string{"coverage", ":d_test"}, extraArgs...)...); err != nil {
		t.Fatal(err)
	}
}

func TestCoverageWithCorrectLineNumbers(t *testing.T) {
	t.Run("lcov", func(t *testing.T) {
		testCoverageWithCorrectLineNumbers(t)
	})
	t.Run("go_cover", func(t *testing.T) {
		testCoverageWithCorrectLineNumbers(t, "--@io_bazel_rules_go//go/config:cover_format=go_cover")
	})
}

func testCoverageWithCorrectLineNumbers(t *testing.T, extraArgs ...string) {
	if err := bazel_testing.RunBazel(append([]string{"coverage", ":panicking_test"}, extraArgs...)...); err != nil {
		t.Fatal(err)
	}
}
