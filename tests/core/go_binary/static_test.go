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

// +build linux

package static_cgo_test

import (
	"debug/elf"
	"os"
	"testing"

	"github.com/bazelbuild/rules_go/go/tools/bazel"
)

func TestStatic(t *testing.T) {
	for _, name := range []string{"static_bin", "static_cgo_bin", "static_cross_bin", "static_pure_bin"} {
		if name != "static_pure_bin" && os.Getenv("ZIG_CC") == "1" {
			// zig does not statically link glibc, by design or accident.
			t.Skip()
		}

		t.Run(name, func(t *testing.T) {
			path, ok := bazel.FindBinary("tests/core/go_binary", name)
			if !ok {
				t.Fatal("could not find static_cgo_bin")
			}
			f, err := elf.Open(path)
			if err != nil {
				t.Fatal(err)
			}
			defer f.Close()
			for _, prog := range f.Progs {
				if prog.Type == elf.PT_INTERP {
					t.Fatalf("binary %s has PT_INTERP segment, indicating dynamic linkage", path)
				}
			}
		})
	}
}
