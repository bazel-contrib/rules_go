// Copyright 2023 The Bazel Authors. All rights reserved.
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

package pgo_test

import (
	_ "embed"
	"os"
	"path"
	"reflect"
	"strings"
	"testing"

	"github.com/bazelbuild/rules_go/go/tools/bazel_testing"
)

//go:embed pgo.pprof
var pgoProfile []byte

func TestMain(m *testing.M) {
	bazel_testing.TestMain(m, bazel_testing.Args{
		Main: `
-- src/BUILD.bazel --
load("@io_bazel_rules_go//go:def.bzl", "go_binary", "go_test")

go_binary(
    name = "pgo_with_profile",
    srcs = ["pgo.go"],
    pgoprofile = ":pgo.pprof",
)

go_binary(
    name = "pgo_with_preprocessed_profile",
    srcs = ["pgo.go"],
    pgoprofile = ":pgo.preprofile",
)

go_binary(
    name = "pgo_without_profile",
    srcs = ["pgo.go"],
)

-- src/pgo.go --
package main

import "fmt"

func main() {
  fmt.Println("Did you know that profile guided optimization was added to the go compiler in go version 1.20?")
}

-- src/pgo.preprofile --
GO PREPROFILE V1
main.caller
main.callee
1 100
`,
	})
}

// writeProfile writes the pprof file used by the test targets.
//
// This must be done here rather than in the txtar archive above as txtar
// changes the content of the pprof file and it could not be parsed.
func writeProfile(t *testing.T) {
	t.Helper()
	pwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path.Join(pwd, "src", "pgo.pprof"), pgoProfile, 0644); err != nil {
		t.Fatal(err)
	}
}

func TestGoBinaryOutputWithPgoProfileDiffersFromGoBinaryWithoutPgoProfile(t *testing.T) {
	writeProfile(t)

	// Ensure both targets can be built
	if err := bazel_testing.RunBazel("build", "//src:pgo_with_profile", "//src:pgo_without_profile"); err != nil {
		t.Fatal(err)
	}

	// Get the paths to the two binaries.
	var out []byte
	var stderr []byte
	var err error
	if out, stderr, err = bazel_testing.BazelOutputWithInput(nil, "cquery", "--output=files", "//src:pgo_with_profile", "//src:pgo_without_profile"); err != nil {
		t.Fatal(err)
	}
	files := strings.Split(strings.TrimSpace(string(out)), "\n")
	if len(files) != 2 {
		t.Fatalf("expected 2 files, got %+v:\n%s", files, string(stderr))
	}

	// Verify that the binaries differs.
	firstBinary, err := os.ReadFile(files[0])
	if err != nil {
		t.Fatal(err)
	}
	secondBinary, err := os.ReadFile(files[1])
	if err != nil {
		t.Fatal(err)
	}

	if reflect.DeepEqual(firstBinary, secondBinary) {
		t.Fatal("the two binaries are equal when they should be different")
	}
}

// TestPgoProfileIsPreprocessed verifies that the pprof profile is converted into
// the compiler's intermediate representation before it reaches any compiler.
func TestPgoProfileIsPreprocessed(t *testing.T) {
	writeProfile(t)

	if err := bazel_testing.RunBazel("build", "//src:pgo_with_profile"); err != nil {
		t.Fatal(err)
	}

	converted := actionOutputs(aquery(t, `mnemonic("GoPreprofile", deps(//src:pgo_with_profile))`))
	if len(converted) == 0 {
		t.Fatal("expected a GoPreprofile action")
	}

	// The action appears in every configuration it is reachable from, but only
	// the outputs this build needed have been written.
	checked := 0
	for _, out := range converted {
		content, err := os.ReadFile(out)
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			t.Fatal(err)
		}
		checked++
		if got := firstLine(string(content)); got != preprofileHeader {
			t.Errorf("%s starts with %q, want %q: update preprofileHeader and the src/pgo.preprofile fixture, and check that preprofilePrefix in go/tools/builders/preprofile.go still matches",
				out, got, preprofileHeader)
		}
	}
	if checked == 0 {
		t.Fatalf("none of the GoPreprofile outputs %v were built", converted)
	}

	// No compiler may be handed the raw pprof profile.
	actions := aquery(t, `mnemonic("GoCompilePkg|GoStdlib", deps(//src:pgo_with_profile))`)
	profiles := pgoProfileArgs(actions)
	if len(profiles) == 0 {
		t.Fatalf("expected some compile actions to use a profile, got:\n%s", actions)
	}
	for _, profile := range profiles {
		if !strings.HasSuffix(profile, ".preprofile") {
			t.Errorf("compile action got raw profile %s, want a preprocessed one", profile)
		}
	}
}

// TestPreprocessedPgoProfileIsAccepted verifies that a profile that has already
// been converted with "go tool preprofile" is passed through unchanged. The
// conversion tool rejects its own output, so converting it again would fail the
// build.
func TestPreprocessedPgoProfileIsAccepted(t *testing.T) {
	writeProfile(t)

	if err := bazel_testing.RunBazel("build", "//src:pgo_with_preprocessed_profile"); err != nil {
		t.Fatal(err)
	}
}

// preprofileHeader is the first line "go tool preprofile" currently writes. The
// builder only matches its "GO PREPROFILE " prefix, so keep this one exact: a
// version bump is not reported by anything else in the build.
const preprofileHeader = "GO PREPROFILE V1"

func firstLine(s string) string {
	line, _, _ := strings.Cut(s, "\n")
	return line
}

func aquery(t *testing.T, query string) string {
	t.Helper()
	out, stderr, err := bazel_testing.BazelOutputWithInput(nil, "aquery", query)
	if err != nil {
		t.Fatalf("aquery %s: %v\n%s", query, err, string(stderr))
	}
	return string(out)
}

// actionOutputs returns the outputs of every action in the text output of an
// aquery, which prints them as a bracketed, comma separated list.
func actionOutputs(aqueryOutput string) []string {
	var outputs []string
	for _, line := range strings.Split(aqueryOutput, "\n") {
		list, ok := strings.CutPrefix(strings.TrimSpace(line), "Outputs: [")
		if !ok {
			continue
		}
		list, ok = strings.CutSuffix(list, "]")
		if !ok {
			continue
		}
		for _, output := range strings.Split(list, ", ") {
			outputs = append(outputs, output)
		}
	}
	return outputs
}

// pgoProfileArgs returns the value of every -pgoprofile argument in the text
// output of an aquery, which prints one argument per line, indented and with a
// trailing line continuation or a closing parenthesis on the final one.
func pgoProfileArgs(aqueryOutput string) []string {
	trimArg := func(line string) string {
		return strings.Trim(line, " \t\\)")
	}

	lines := strings.Split(aqueryOutput, "\n")
	var profiles []string
	for i, line := range lines {
		if trimArg(line) != "-pgoprofile" || i+1 == len(lines) {
			continue
		}
		profiles = append(profiles, trimArg(lines[i+1]))
	}
	return profiles
}
