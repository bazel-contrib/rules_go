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

package pathmapping_test

import (
	"encoding/json"
	"fmt"
	"regexp"
	"slices"
	"testing"

	"github.com/bazelbuild/rules_go/go/tools/bazel_testing"
)

func TestMain(m *testing.M) {
	bazel_testing.TestMain(m, bazel_testing.Args{
		Main: `
-- BUILD.bazel --
load("@io_bazel_rules_go//go:def.bzl", "go_binary", "go_library")

go_library(
    name = "hello_lib",
    srcs = ["hello.go"],
    importpath = "example.com/hello",
)

go_binary(
    name = "hello",
    embed = [":hello_lib"],
)

-- hello.go --
package hello

import "fmt"

func Hello() { fmt.Println("hello") }
`,
	})
}

// unstrippedConfigSegment matches a path segment right after "bazel-out/"
// that carries configuration information (e.g. "darwin_arm64-fastbuild",
// "k8-opt-exec-ST-<hash>"). Path-mapped arguments replace this segment with
// the fixed placeholder "cfg", so a match here means the arg was not path
// mapped.
var unstrippedConfigSegment = regexp.MustCompile(`bazel-out/[^/]*-(fastbuild|dbg|opt)[^/]*/`)

func TestSdkArgIsPathMapped(t *testing.T) {
	cases := []struct {
		mnemonic string
		target   string
	}{
		{"GoCompilePkg", "//:hello_lib"},
		{"GoInfo", "@io_bazel_rules_go//:go_info"},
	}
	for _, c := range cases {
		t.Run(c.mnemonic, func(t *testing.T) {
			stripped := aqueryArgs(t, c.mnemonic, c.target, "--experimental_output_paths=strip")
			assertPathMapped(t, stripped, "-sdk")
			// GoInfo does not pass -goroot; only assert it for actions that do.
			if _, ok := flagValue(stripped, "-goroot"); ok {
				assertPathMapped(t, stripped, "-goroot")
				// Sanity check: without path mapping, -goroot points to a
				// bazel-out path with a real config segment. This makes sure
				// the regex used by assertPathMapped would actually catch a
				// regression if -goroot (or -sdk) stopped being path mapped.
				unstripped := aqueryArgs(t, c.mnemonic, c.target)
				assertMatchesConfigSegment(t, unstripped, "-goroot")
			}
		})
	}
}

// aqueryArgs runs bazel aquery --output=jsonproto and returns the arguments
// of the (single) matching action.
func aqueryArgs(t *testing.T, mnemonic, target string, extraFlags ...string) []string {
	t.Helper()
	cmd := []string{
		"aquery",
		"--include_commandline",
		"--include_param_files",
		"--output=jsonproto",
	}
	cmd = append(cmd, extraFlags...)
	cmd = append(cmd, fmt.Sprintf(`mnemonic("%s", %s)`, mnemonic, target))
	out, err := bazel_testing.BazelOutput(cmd...)
	if err != nil {
		t.Fatalf("bazel aquery failed: %v", err)
	}
	var parsed struct {
		Actions []struct {
			Arguments []string `json:"arguments"`
		} `json:"actions"`
	}
	if err := json.Unmarshal(out, &parsed); err != nil {
		t.Fatalf("failed to decode aquery output: %v\n%s", err, out)
	}
	if len(parsed.Actions) != 1 {
		t.Fatalf("expected 1 %s action for %s, got %d", mnemonic, target, len(parsed.Actions))
	}
	return parsed.Actions[0].Arguments
}

// assertPathMapped verifies that the value following the given flag does not
// contain a bazel-out path with an unstripped configuration segment. Values
// that come through path mapping are rewritten to use the fixed "bazel-out/cfg/"
// prefix. Non-bazel-out values (source paths under external/, etc.) are left
// alone by both the mapping and this check.
func assertPathMapped(t *testing.T, args []string, flag string) {
	t.Helper()
	val, ok := flagValue(args, flag)
	if !ok {
		t.Fatalf("flag %q not found in args %v", flag, args)
	}
	if unstrippedConfigSegment.MatchString(val) {
		t.Fatalf("flag %q value %q contains an unstripped config segment; "+
			"the argument must be produced via args.add_all(..., map_each = _dirname) "+
			"so that path mapping applies", flag, val)
	}
}

// assertMatchesConfigSegment verifies the flag's value is a bazel-out path
// with a config segment. It's used to sanity-check that
// unstrippedConfigSegment matches real Bazel output, so that a broken regex
// doesn't turn assertPathMapped into a no-op.
func assertMatchesConfigSegment(t *testing.T, args []string, flag string) {
	t.Helper()
	val, ok := flagValue(args, flag)
	if !ok {
		t.Fatalf("flag %q not found in args %v", flag, args)
	}
	if !unstrippedConfigSegment.MatchString(val) {
		t.Fatalf("expected flag %q value %q to contain a config segment "+
			"(without --experimental_output_paths=strip); the regex may be "+
			"out of date", flag, val)
	}
}

// flagValue returns the argument that follows flag, or false if flag is not
// present.
func flagValue(args []string, flag string) (string, bool) {
	i := slices.Index(args, flag)
	if i < 0 || i+1 >= len(args) {
		return "", false
	}
	return args[i+1], true
}
