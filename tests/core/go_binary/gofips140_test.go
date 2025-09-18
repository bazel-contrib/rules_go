// Copyright 2025 The Bazel Authors. All rights reserved.
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

package gofips140_test

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
    name = "gofips140_off",
    srcs = ["gofips140.go"],
    gofips140 = "off",
)

go_binary(
    name = "gofips140_latest",
    srcs = ["gofips140.go"],
    gofips140 = "latest",
)

go_binary(
    name = "gofips140_version",
    srcs = ["gofips140.go"],
    gofips140 = "v1.0.0",
)

-- gofips140.go --
package main

import (
	"crypto/fips140"
	"fmt"
)

func main() {
	fmt.Printf("%t", fips140.Enabled())
}
`,
	})
}

// TestGOFIPS140Attribute checks that the gofips140 attribute on go_binary
// controls the GOFIPS140 environment variable.
func TestGOFIPS140Attribute(t *testing.T) {
	tests := []struct {
		target string
		want   string
	}{
		{"//:gofips140_off", "false"},
		{"//:gofips140_latest", "true"},
		{"//:gofips140_version", "true"},
	}

	for _, tt := range tests {
		t.Run(tt.target, func(t *testing.T) {
			out, err := bazel_testing.BazelOutput("run", tt.target)
			if err != nil {
				t.Fatalf("running %s: %v", tt.target, err)
			}
			got := string(bytes.TrimSpace(out))
			if got != tt.want {
				t.Fatalf("got %q; want %q", got, tt.want)
			}
		})
	}
}
