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

package bind_now_test

import (
	"debug/elf"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/bazelbuild/rules_go/go/tools/bazel_testing"
)

func TestMain(m *testing.M) {
	bazel_testing.TestMain(m, bazel_testing.Args{
		Main: `
-- src/go.mod --
module example.com/hello

go 1.21

-- src/main.go --
package main

import "C"

func main() {}

-- src/BUILD.bazel --
load("@io_bazel_rules_go//go:def.bzl", "go_binary")

go_binary(
    name = "hello_auto",
    srcs = ["main.go"],
    cgo = True,
)

go_binary(
    name = "hello_pie",
    srcs = ["main.go"],
    cgo = True,
    linkmode = "pie",
)
`,
	})
}

// TestBindNowConsistentWithGoBuild verifies that rules_go produces binaries
// with the same BIND_NOW behavior as native "go build". The CC toolchain
// often passes -Wl,-z,relro,-z,now which sets BIND_NOW, breaking Go libraries
// that use dlopen/dlsym at runtime (e.g., NVIDIA's go-nvml). See #4377.
func TestBindNowConsistentWithGoBuild(t *testing.T) {
	tests := []struct {
		name        string
		bazelTarget string
		goBuildArgs []string
	}{
		{"auto", "//src:hello_auto", nil},
		{"pie", "//src:hello_pie", []string{"-buildmode=pie"}},
	}

	// Build all bazel targets.
	if err := bazel_testing.RunBazel("build", "//src:hello_auto", "//src:hello_pie"); err != nil {
		t.Fatal(err)
	}

	// Locate the Go SDK from the output base.
	out, err := bazel_testing.BazelOutput("info", "output_base")
	if err != nil {
		t.Fatal(err)
	}
	outputBase := strings.TrimSpace(string(out))
	goCmd := filepath.Join(outputBase, "external", "go_sdk", "bin", "go")

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Get bazel binary path.
			bout, _, err := bazel_testing.BazelOutputWithInput(nil, "cquery", "--output=files", tt.bazelTarget)
			if err != nil {
				t.Fatal(err)
			}
			bazelBin := strings.TrimSpace(string(bout))

			// Build with go build using the same SDK.
			tmpDir := t.TempDir()
			goBin := filepath.Join(tmpDir, "hello")
			wd, err := os.Getwd()
			if err != nil {
				t.Fatal(err)
			}

			args := append([]string{"build", "-o", goBin}, tt.goBuildArgs...)
			args = append(args, ".")
			cmd := exec.Command(goCmd, args...)
			cmd.Dir = filepath.Join(wd, "src")
			cmd.Env = append(os.Environ(),
				"CGO_ENABLED=1",
				"GOCACHE="+filepath.Join(tmpDir, "gocache"),
			)
			cmd.Stderr = os.Stderr
			if err := cmd.Run(); err != nil {
				t.Fatalf("go build failed: %v", err)
			}

			// Compare BIND_NOW flags.
			bazelBindNow, err := hasBindNow(bazelBin)
			if err != nil {
				t.Fatalf("failed to check bazel binary: %v", err)
			}
			goBindNow, err := hasBindNow(goBin)
			if err != nil {
				t.Fatalf("failed to check go binary: %v", err)
			}

			if bazelBindNow != goBindNow {
				t.Errorf("BIND_NOW mismatch: bazel binary has BIND_NOW=%v, go build binary has BIND_NOW=%v",
					bazelBindNow, goBindNow)
			}
		})
	}
}

// hasBindNow checks if an ELF binary has BIND_NOW set in its dynamic section.
func hasBindNow(path string) (bool, error) {
	f, err := elf.Open(path)
	if err != nil {
		return false, err
	}
	defer f.Close()

	ds := f.SectionByType(elf.SHT_DYNAMIC)
	if ds == nil {
		return false, nil
	}
	d, err := ds.Data()
	if err != nil {
		return false, err
	}

	entSize := 16
	if f.Class == elf.ELFCLASS32 {
		entSize = 8
	}
	for i := 0; i+entSize <= len(d); i += entSize {
		var tag, val uint64
		if f.Class == elf.ELFCLASS64 {
			tag = f.ByteOrder.Uint64(d[i:])
			val = f.ByteOrder.Uint64(d[i+8:])
		} else {
			tag = uint64(f.ByteOrder.Uint32(d[i:]))
			val = uint64(f.ByteOrder.Uint32(d[i+4:]))
		}

		if elf.DynTag(tag) == elf.DT_FLAGS && elf.DynFlag(val)&elf.DF_BIND_NOW != 0 {
			return true, nil
		}
		if elf.DynTag(tag) == elf.DT_FLAGS_1 && elf.DynFlag1(val)&elf.DF_1_NOW != 0 {
			return true, nil
		}
	}
	return false, nil
}
