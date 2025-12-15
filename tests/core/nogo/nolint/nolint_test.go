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

package nolint_test

import (
	"bytes"
	"io/ioutil"
	"strings"
	"testing"

	"github.com/bazelbuild/rules_go/go/tools/bazel_testing"
)

func TestMain(m *testing.M) {
	bazel_testing.TestMain(m, bazel_testing.Args{
		Nogo: "@//:nogo",
		Main: `
-- BUILD.bazel --
load("@io_bazel_rules_go//go:def.bzl", "go_library", "go_tool_library", "nogo")

nogo(
    name = "nogo",
    vet = True,
    deps = ["@org_golang_x_tools//go/analysis/passes/nilness"],
    visibility = ["//visibility:public"],
)

go_library(
    name = "inline",
    srcs = ["inline.go"],
    importpath = "test",
)

go_library(
    name = "inline_filter",
    srcs = ["inline_filter.go"],
    importpath = "test",
)

go_library(
    name = "block",
    srcs = ["block.go"],
    importpath = "test",
)

go_library(
    name = "block_multiline",
    srcs = ["block_multiline.go"],
    importpath = "test",
)

go_library(
    name = "inline_errors",
    srcs = ["inline_errors.go"],
    importpath = "test",
)

go_library(
    name = "inline_column",
		srcs = ["inline_column.go"],
		importpath = "test",
)

go_library(
    name = "large_block",
		srcs = ["large_block.go"],
		importpath = "test",
)
-- inline.go --
package test

import "fmt"

func F() {
	s := "hello"
	fmt.Printf("%d", s) //nolint
}

-- inline_filter.go --
package test

func F() bool {
	return true || true //nolint:bools
}

-- block.go --
package test

import "fmt"

func F() {
	//nolint
	fmt.Printf("%d", "hello")
}

-- block_multiline.go --
package test

func F() bool {
	var i *int
	//nolint
	return true &&
		i != nil
}

-- inline_errors.go --
package test

import "fmt"

func F() {
	var i *int
	if i == nil {
		fmt.Printf("%d", "hello") //nolint
		fmt.Println(*i)           // Keep nil deref error
	}
}

-- inline_column.go --
package test

import "fmt"

func F() {
	// Purposely used 'helo' to align the column
	fmt.Printf("%d", "helo") //nolint
	superLongVariableName := true || true
	var _ = superLongVariableName
}

-- large_block.go --
package test

import "fmt"

var V = struct {
	S string
	B bool
} {
	S: fmt.Sprintf("%d", "hello"), //nolint
	B: true || true,
}
`,
	})
}

func Test(t *testing.T) {
	tests := []struct {
		Name     string
		Target   string
		Expected string
	}{
		{
			Name:   "Inline comment",
			Target: "//:inline",
		},
		{
			Name:   "Inline with lint filter",
			Target: "//:inline_filter",
		},
		{
			Name:   "Block comment",
			Target: "//:block",
		},
		{
			Name:   "Multiline block comment",
			Target: "//:block_multiline",
		},
		{
			Name:     "Inline with errors",
			Target:   "//:inline_errors",
			Expected: "inline_errors.go:9:15: nil dereference in load (nilness)",
		},
		{
			Name:     "Inline comment on same column does not apply",
			Target:   "//:inline_column",
			Expected: "inline_column.go:8:27: redundant or: true || true (bools)",
		},
		{
			Name:     "Inline comment does not apply to larger block",
			Target:   "//:large_block",
			Expected: "large_block.go:10:5: redundant or: true || true (bools)",
		},
	}

	for _, tc := range tests {
		t.Run(tc.Name, func(t *testing.T) {
			cmd := bazel_testing.BazelCmd("build", tc.Target)
			b, err := cmd.CombinedOutput()
			output := string(b)
			if tc.Expected != "" && err == nil {
				t.Fatal("unexpected success", output)
			}
			if tc.Expected == "" && err != nil {
				t.Fatal("unexpected failure", output)
			}
			if !strings.Contains(output, tc.Expected) {
				t.Errorf("output did not contain expected: %s\n%s", tc.Expected, output)
			}
		})
	}
}

func replaceInFile(path, old, new string) error {
	data, err := ioutil.ReadFile(path)
	if err != nil {
		return err
	}
	data = bytes.ReplaceAll(data, []byte(old), []byte(new))
	return ioutil.WriteFile(path, data, 0666)
}
