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
		t.Fatal(err)
	}
	t.Setenv("RULES_GO_TEST_C_COMPILER_FLAGS", supported)
	t.Setenv("RULES_GO_TEST_C_COMPILER_DIAGNOSTIC", diagnostic)
	return cc
}

func TestDefaultCFlags(t *testing.T) {
	t.Setenv("GOOS", "linux")
	for _, tc := range []struct {
		name      string
		supported string
		prefix    string
	}{
		{"prefer_file", "-ffile-prefix-map=a=b -fdebug-prefix-map=a=b", "-ffile-prefix-map"},
		{"fallback_debug", "-fdebug-prefix-map=a=b", "-fdebug-prefix-map"},
		{"neither_supported", "", ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			workDir := t.TempDir()
			cc := fakeCCompiler(t, tc.supported, "unrecognized command-line option")
			var want []string
			if tc.prefix != "" {
				want = append(want, tc.prefix+"="+abs(".")+"=.", tc.prefix+"="+workDir+"=.")
			}
			want = append(want, "-pthread")
			if got := defaultCFlags(workDir, cc); !reflect.DeepEqual(got, want) {
				t.Errorf("defaultCFlags() = %q, want %q", got, want)
			}
		})
	}
}

func TestCCompilerSupportsFlag(t *testing.T) {
	for _, tc := range []struct {
		diagnostic string
		want       bool
	}{
		{"", true},
		{"unrecognized command-line option", false},
		{"unknown argument", false},
		{"unrecognised option", false},
		{"option is not supported", false},
		{"option not recognized", false},
		{"unsupported option", false},
		// An unrelated compilation failure does not imply an unsupported flag.
		{"fatal error: cannot create output file", true},
	} {
		t.Run(tc.diagnostic, func(t *testing.T) {
			cc := fakeCCompiler(t, "", tc.diagnostic)
			if got := cCompilerSupportsFlag(t.TempDir(), cc, "-ffile-prefix-map=a=b"); got != tc.want {
				t.Errorf("cCompilerSupportsFlag() = %t, want %t", got, tc.want)
			}
		})
	}
}
