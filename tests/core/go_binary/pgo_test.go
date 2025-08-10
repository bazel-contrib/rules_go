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
    name = "pgo_without_profile",
    srcs = ["pgo.go"],
)

-- src/pgo.go --
package main

import "fmt"

func main() {
  fmt.Println("Did you know that profile guided optimization was added to the go compiler in go version 1.20?")
}
`,
	})
}

func TestGoBinaryOutputWithPgoProfileDiffersFromGoBinaryWithoutPgoProfile(t *testing.T) {
	// Write the pgo.pprof file.
	// This must be done as txtar changes the content of the pprof file and it could not be parsed.
	pwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path.Join(pwd, "src", "pgo.pprof"), pgoProfile, 0644); err != nil {
		t.Fatal(err)
	}

	// Ensure both targets can be built
	if err := bazel_testing.RunBazel("build", "//src:all"); err != nil {
		t.Fatal(err)
	}

	// Get the paths to the two binaries.
	var out []byte
	var stderr []byte
	if out, stderr, err = bazel_testing.BazelOutputWithInput(nil, "cquery", "--output=files", "//src:all"); err != nil {
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
