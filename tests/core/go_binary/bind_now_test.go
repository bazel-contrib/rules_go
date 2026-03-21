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

//go:build linux

package test

import (
	"debug/elf"
	"testing"
)

// TestNoBindNow verifies that cgo binaries are not linked with BIND_NOW.
// The CC toolchain often passes -Wl,-z,relro,-z,now to the linker, but
// -z,now (BIND_NOW) breaks Go libraries that use dlopen/dlsym to load
// symbols at runtime (e.g., NVIDIA's go-nvml). See #4377.
func TestNoBindNow(t *testing.T) {
	for _, name := range []string{"hello_pie_bin", "hello_auto_bin"} {
		t.Run(name, func(t *testing.T) {
			e, err := openELF("tests/core/go_binary", name)
			if err != nil {
				t.Fatal(err)
			}

			ds := e.SectionByType(elf.SHT_DYNAMIC)
			if ds == nil {
				// Statically linked binary, no dynamic section to check.
				return
			}
			d, err := ds.Data()
			if err != nil {
				t.Fatal(err)
			}

			// Parse dynamic entries to check for DF_BIND_NOW (in DT_FLAGS)
			// and DF_1_NOW (in DT_FLAGS_1).
			entSize := 16 // sizeof(Elf64_Dyn) on 64-bit
			if e.Class == elf.ELFCLASS32 {
				entSize = 8
			}
			for i := 0; i+entSize <= len(d); i += entSize {
				var tag, val uint64
				if e.Class == elf.ELFCLASS64 {
					tag = e.ByteOrder.Uint64(d[i:])
					val = e.ByteOrder.Uint64(d[i+8:])
				} else {
					tag = uint64(e.ByteOrder.Uint32(d[i:]))
					val = uint64(e.ByteOrder.Uint32(d[i+4:]))
				}

				// DT_FLAGS = 30, DF_BIND_NOW = 0x8
				if tag == 30 && val&0x8 != 0 {
					t.Errorf("binary %s has DF_BIND_NOW set in DT_FLAGS", name)
				}
				// DT_FLAGS_1 = 0x6ffffffb, DF_1_NOW = 0x1
				if tag == 0x6ffffffb && val&0x1 != 0 {
					t.Errorf("binary %s has DF_1_NOW set in DT_FLAGS_1", name)
				}
			}
		})
	}
}
