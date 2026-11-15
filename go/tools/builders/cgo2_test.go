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

package main

import (
	"fmt"
	"os"
	"reflect"
	"strings"
	"testing"
)

// Re-execute the test binary as a compiler, without requiring a C toolchain or
// a shell. TestMain runs before the compiler arguments are parsed as test flags.
func TestMain(m *testing.M) {
	if supported, ok := os.LookupEnv("RULES_GO_TEST_C_COMPILER_FLAGS"); ok {
		for _, flag := range strings.Fields(supported) {
			if os.Args[1] == flag {
				os.Exit(0)
			}
		}
		fmt.Fprintln(os.Stderr, os.Getenv("RULES_GO_TEST_C_COMPILER_DIAGNOSTIC"))
		os.Exit(1)
	}
	os.Exit(m.Run())
}

func fakeCCompiler(t *testing.T, supported, diagnostic string) string {
	t.Helper()
	cc, err := os.Executable()
	if err != nil {
		t.Fatalf("os.Executable: %v", err)
	}
	t.Setenv("RULES_GO_TEST_C_COMPILER_FLAGS", supported)
	t.Setenv("RULES_GO_TEST_C_COMPILER_DIAGNOSTIC", diagnostic)
	return cc
}

func TestDefaultCFlags(t *testing.T) {
	t.Setenv("GOOS", "linux")
	testCases := []struct {
		name      string
		supported string
		prefix    string
	}{
		{
			name:      "prefer file prefix map",
			supported: "-ffile-prefix-map=a=b -fdebug-prefix-map=a=b",
			prefix:    "-ffile-prefix-map",
		},
		{
			name:      "fall back to debug prefix map",
			supported: "-fdebug-prefix-map=a=b",
			prefix:    "-fdebug-prefix-map",
		},
		{
			name: "neither flag supported",
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			workDir := t.TempDir()
			cc := fakeCCompiler(t, tc.supported, "unrecognized command-line option")
			var want []string
			if tc.prefix != "" {
				want = append(want, tc.prefix+"="+abs(".")+"=.", tc.prefix+"="+workDir+"=.")
			}
			want = append(want, "-pthread")
			if got := defaultCFlags(workDir, cc); !reflect.DeepEqual(got, want) {
				t.Errorf("defaultCFlags(%q, %q) = %q, want %q", workDir, cc, got, want)
			}
		})
	}
}

func TestCCompilerSupportsFlag(t *testing.T) {
	testCases := []struct {
		name       string
		diagnostic string
		want       bool
	}{
		{name: "no diagnostic", want: true},
		{name: "unrecognized", diagnostic: "unrecognized command-line option"},
		{name: "unknown", diagnostic: "unknown argument"},
		{name: "unrecognised", diagnostic: "unrecognised option"},
		{name: "not supported", diagnostic: "option is not supported"},
		{name: "not recognized", diagnostic: "option not recognized"},
		{name: "unsupported", diagnostic: "unsupported option"},
		// An unrelated compilation failure does not imply an unsupported flag.
		{name: "unrelated error", diagnostic: "fatal error: cannot create output file", want: true},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			cc := fakeCCompiler(t, "", tc.diagnostic)
			if got := cCompilerSupportsFlag(t.TempDir(), cc, "-ffile-prefix-map=a=b"); got != tc.want {
				t.Errorf("cCompilerSupportsFlag() = %t, want %t", got, tc.want)
			}
		})
	}
}
