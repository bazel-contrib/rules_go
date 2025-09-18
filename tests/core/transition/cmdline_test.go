// Copyright 2020 The Bazel Authors. All rights reserved.
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

package cmdline_test

import (
	"bytes"
	"testing"

	"github.com/bazelbuild/rules_go/go/tools/bazel_testing"
)

func TestMain(m *testing.M) {
	bazel_testing.TestMain(m, bazel_testing.Args{
		Main: `
-- BUILD.bazel --
load("@io_bazel_rules_go//go:def.bzl", "go_binary")

go_binary(
    name = "maybe_pure",
    srcs = [
        "not_pure.go",
        "pure.go",
    ],
)

go_binary(
    name = "gofips140_test",
    srcs = ["gofips140.go"],
)

-- not_pure.go --
// +build cgo

package main

import "fmt"

func main() {
	fmt.Println("not pure")
}

-- pure.go --
// +build !cgo

package main

import "fmt"

func main() {
	fmt.Println("pure")
}

-- gofips140.go --
package main

import (
	"fmt"
	"os"
)

func main() {
	gofips140 := os.Getenv("GOFIPS140")
	fmt.Printf("GOFIPS140=%s", gofips140)
}
`,
	})
}

// TestPure checks that the --@io_bazel_rules_go//go/config:pure flag controls
// whether a target is built in pure mode. It doesn't actually require cgo,
// since that doesn't work within go_bazel_test on Windows.
func TestPure(t *testing.T) {
	out, err := bazel_testing.BazelOutput("run", "//:maybe_pure")
	if err != nil {
		t.Fatalf("running //:maybe_pure without flag: %v", err)
	}
	got := string(bytes.TrimSpace(out))
	if want := "not pure"; got != want {
		t.Fatalf("got %q; want %q", got, want)
	}

	out, err = bazel_testing.BazelOutput("run", "--@io_bazel_rules_go//go/config:pure", "//:maybe_pure")
	if err != nil {
		t.Fatalf("running //:maybe_pure with flag: %v", err)
	}
	got = string(bytes.TrimSpace(out))
	if want := "pure"; got != want {
		t.Fatalf("got %q; want %q", got, want)
	}
}

// TestGOFIPS140 checks that the --@io_bazel_rules_go//go/config:gofips140 flag
// controls the GOFIPS140 environment variable.
func TestGOFIPS140(t *testing.T) {
	// Test default value (should be "off")
	out, err := bazel_testing.BazelOutput("run", "//:gofips140_test")
	if err != nil {
		t.Fatalf("running //:gofips140_test without flag: %v", err)
	}
	got := string(bytes.TrimSpace(out))
	if want := "GOFIPS140=off"; got != want {
		t.Fatalf("got %q; want %q", got, want)
	}

	// Test with "latest" value
	out, err = bazel_testing.BazelOutput("run", "--@io_bazel_rules_go//go/config:gofips140=latest", "//:gofips140_test")
	if err != nil {
		t.Fatalf("running //:gofips140_test with gofips140=latest: %v", err)
	}
	got = string(bytes.TrimSpace(out))
	if want := "GOFIPS140=latest"; got != want {
		t.Fatalf("got %q; want %q", got, want)
	}

	// Test with specific version
	out, err = bazel_testing.BazelOutput("run", "--@io_bazel_rules_go//go/config:gofips140=v1.0.0", "//:gofips140_test")
	if err != nil {
		t.Fatalf("running //:gofips140_test with gofips140=v1.0.0: %v", err)
	}
	got = string(bytes.TrimSpace(out))
	if want := "GOFIPS140=v1.0.0"; got != want {
		t.Fatalf("got %q; want %q", got, want)
	}
}
