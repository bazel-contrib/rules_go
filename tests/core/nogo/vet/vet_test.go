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

package vet_test

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"regexp"
	"testing"

	"github.com/bazelbuild/rules_go/go/tools/bazel_testing"
)

func TestMain(m *testing.M) {
	bazel_testing.TestMain(m, bazel_testing.Args{
		Nogo: "@io_bazel_rules_go//:default_nogo",
		Main: `
-- BUILD.bazel --
load("@io_bazel_rules_go//go:def.bzl", "go_library", "go_tool_library", "nogo")

nogo(
    name = "nogo",
    vet = True,
    visibility = ["//visibility:public"],
)

go_library(
    name = "has_errors",
    srcs = ["has_errors.go"],
    importpath = "haserrors",
    deps = [":fmtwrap"],
)

go_library(
    name = "no_errors",
    srcs = ["no_errors.go"],
    cgo = True,
    importpath = "noerrors",
)

go_library(
    name = "fmtwrap",
    srcs = ["fmtwrap.go"],
    importpath = "fmtwrap",
)

-- has_errors.go --
package haserrors

// +build build_tags_error

import (
	"fmtwrap"
	"sync/atomic"
)

func F() {}

func Foo() bool {
	x := uint64(1)
	_ = atomic.AddUint64(&x, 1)
	if F == nil { // nilfunc error.
		return false
	}
	fmtwrap.Printf("%b", "hi") // printf error.
	return true || true        // redundant boolean error.
}

-- no_errors.go --
package noerrors

// const int x = 1;
import "C"

func Foo() bool {
	return bool(C.x == 1)
}

-- fmtwrap.go --
package fmtwrap

import "fmt"

func Printf(format string, args ...interface{}) {
	fmt.Printf(format, args...)
}
`,
	})
}

func Test(t *testing.T) {
	for _, test := range []struct {
		desc, nogo, target string
		wantSuccess        bool
		includes, excludes []string
	}{
		{
			desc:        "default",
			target:      "//:has_errors",
			wantSuccess: true,
			excludes: []string{
				"\\+build comment must appear before package clause and be followed by a blank line",
				"comparison of function F == nil is always false",
				"Printf format %b has arg \"hi\" of wrong type string",
				"redundant or: true \\|\\| true",
			},
		}, {
			desc:        "enabled_no_errors",
			target:      "//:no_errors",
			wantSuccess: true,
		}, {
			desc:   "enabled_has_errors",
			nogo:   "@//:nogo",
			target: "//:has_errors",
			includes: []string{
				"misplaced \\+build comment",
				"comparison of function F == nil is always false",
				"Printf format %b has arg \"hi\" of wrong type string",
				"redundant or: true \\|\\| true",
			},
		},
	} {
		t.Run(test.desc, func(t *testing.T) {
			if test.nogo != "" {
				origRegister := `nogo = "@io_bazel_rules_go//:default_nogo",`
				customRegister := fmt.Sprintf("nogo = %q,", test.nogo)
				if err := replaceInFile("WORKSPACE", origRegister, customRegister); err != nil {
					t.Fatal(err)
				}
				defer replaceInFile("WORKSPACE", customRegister, origRegister)
			}

			cmd := bazel_testing.BazelCmd("build", test.target)
			stderr := &bytes.Buffer{}
			cmd.Stderr = stderr
			if err := cmd.Run(); err == nil && !test.wantSuccess {
				t.Fatal("unexpected success")
			} else if err != nil && test.wantSuccess {
				t.Fatalf("unexpected error: %v", err)
			}

			for _, pattern := range test.includes {
				if matched, err := regexp.Match(pattern, stderr.Bytes()); err != nil {
					t.Fatal(err)
				} else if !matched {
					t.Errorf("output did not contain pattern: %s", pattern)
				}
			}
			for _, pattern := range test.excludes {
				if matched, err := regexp.Match(pattern, stderr.Bytes()); err != nil {
					t.Fatal(err)
				} else if matched {
					t.Errorf("output contained pattern: %s", pattern)
				}
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
