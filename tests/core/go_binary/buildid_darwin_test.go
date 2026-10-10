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

//go:build darwin

package buildid_darwin_test

import (
	"bytes"
	"debug/macho"
	"os"
	"os/exec"
	"strings"
	"testing"

	"github.com/bazelbuild/rules_go/go/tools/bazel_testing"
)

func TestMain(m *testing.M) {
	bazel_testing.TestMain(m, bazel_testing.Args{
		Main: `
-- src/pure.go --
package main

func main() {}

-- src/cgo.go --
package main

import "C"

func main() {}

-- src/BUILD.bazel --
load("@io_bazel_rules_go//go:def.bzl", "go_binary")

go_binary(
    name = "pure",
    srcs = ["pure.go"],
    pure = "on",
)

go_binary(
    name = "cgo",
    srcs = ["cgo.go"],
    cgo = True,
)
`,
	})
}

func TestContentBuildID(t *testing.T) {
	pure := build(t, "//src:pure")
	cgo := build(t, "//src:cgo")
	for name, uuid := range map[string][]byte{"pure": pure, "cgo": cgo} {
		if uuid == nil || bytes.Count(uuid, []byte{0}) == len(uuid) {
			t.Errorf("%s: LC_UUID %x is not set", name, uuid)
		}
	}
	if bytes.Equal(pure, cgo) {
		t.Errorf("pure and cgo binaries share LC_UUID %x", pure)
	}
}

// build returns the LC_UUID of target after checking that the patched binary
// still runs, which on arm64 requires a valid code signature.
func build(t *testing.T, target string) []byte {
	t.Helper()
	if err := bazel_testing.RunBazel("build", target); err != nil {
		t.Fatal(err)
	}
	out, err := bazel_testing.BazelOutput("cquery", "--output=files", target)
	if err != nil {
		t.Fatal(err)
	}
	path := strings.TrimSpace(string(out))
	if err := exec.Command(path).Run(); err != nil {
		t.Fatalf("running %s: %v", target, err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(data, []byte(strings.Repeat("0", 40))) {
		t.Errorf("%s: Go build ID placeholder was not replaced", target)
	}
	f, err := macho.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	for _, l := range f.Loads {
		raw := l.Raw()
		if f.ByteOrder.Uint32(raw) == 0x1b { // LC_UUID
			return bytes.Clone(raw[8:24])
		}
	}
	return nil
}
