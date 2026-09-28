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

//go:build linux

package buildid_test

import (
	"bytes"
	"crypto/sha256"
	"debug/elf"
	"debug/macho"
	"encoding/binary"
	"encoding/hex"
	"os"
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

-- src/cdep.go --
package main

// int value(void);
import "C"

func main() { println(C.value()) }

-- src/native.c --
#ifndef VALUE
#define VALUE 1
#endif
int value(void) { return VALUE; }

-- src/BUILD.bazel --
load("@io_bazel_rules_go//go:def.bzl", "go_binary")
load("@rules_cc//cc:cc_library.bzl", "cc_library")

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

cc_library(
    name = "native",
    srcs = ["native.c"],
)

go_binary(
    name = "cdep",
    srcs = ["cdep.go"],
    cdeps = [":native"],
    cgo = True,
)

go_binary(
    name = "custom",
    srcs = ["pure.go"],
    gc_linkopts = ["-buildid=custom"],
    pure = "on",
)
`,
	})
}

type buildIDs struct {
	goID, gnuID string
}

func TestContentBuildID(t *testing.T) {
	pure := build(t, "//src:pure")
	cgo := build(t, "//src:cgo")
	for name, ids := range map[string]buildIDs{"pure": pure, "cgo": cgo} {
		if _, err := hex.DecodeString(ids.goID); err != nil || len(ids.goID) != 40 || ids.goID == strings.Repeat("0", 40) {
			t.Errorf("%s: Go build ID %q is not a content hash", name, ids.goID)
		}
		if ids.gnuID == "" || strings.Trim(ids.gnuID, "0") == "" {
			t.Errorf("%s: GNU build ID %q is not set", name, ids.gnuID)
		}
	}
	if pure == cgo {
		t.Errorf("pure and cgo binaries share build IDs %+v", pure)
	}

	if got := build(t, "//src:custom").goID; got != "custom" {
		t.Errorf("-buildid in gc_linkopts: got Go build ID %q, want %q", got, "custom")
	}
}

func TestNativeChangeChangesBuildID(t *testing.T) {
	v1 := build(t, "//src:cdep", "--copt=-DVALUE=1")
	v2 := build(t, "//src:cdep", "--copt=-DVALUE=2")
	if v1.goID == v2.goID || v1.gnuID == v2.gnuID {
		t.Errorf("changing only native code kept build IDs: %+v and %+v", v1, v2)
	}
}

func TestCrossCompiledMachO(t *testing.T) {
	data, err := os.ReadFile(output(t, "//src:pure", "--platforms=@io_bazel_rules_go//go/toolchain:darwin_arm64"))
	if err != nil {
		t.Fatal(err)
	}
	prefix := []byte("\xff Go build ID: \"")
	i := bytes.Index(data, prefix)
	if i < 0 {
		t.Fatal("Go build ID not found")
	}
	goID := string(data[i+len(prefix):][:40])
	if goID == strings.Repeat("0", 40) {
		t.Fatal("Go build ID placeholder was not replaced")
	}

	f, err := macho.NewFile(bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	uuid := loadCommand(f, 0x1b) // LC_UUID
	if uuid == nil || hex.EncodeToString(uuid[8:14]) != goID[:12] {
		t.Errorf("LC_UUID %x does not match Go build ID %s", uuid, goID)
	}

	// The kernel refuses to run arm64 binaries whose page hashes don't match
	// the code signature.
	cs := loadCommand(f, 0x1d) // LC_CODE_SIGNATURE
	if cs == nil {
		t.Fatal("no code signature")
	}
	be := binary.BigEndian
	sig := data[f.ByteOrder.Uint32(cs[8:]):]
	cd := sig[be.Uint32(sig[16:]):]
	hashOff, nSlots, codeLimit, pageSize := int(be.Uint32(cd[16:])), int(be.Uint32(cd[28:])), int(be.Uint32(cd[32:])), 1<<cd[39]
	for p := range nSlots {
		sum := sha256.Sum256(data[p*pageSize : min((p+1)*pageSize, codeLimit)])
		if !bytes.Equal(sum[:], cd[hashOff+p*sha256.Size:][:sha256.Size]) {
			t.Fatalf("code signature does not match page %d", p)
		}
	}
}

func build(t *testing.T, target string, flags ...string) buildIDs {
	t.Helper()
	f, err := elf.Open(output(t, target, flags...))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	goID := noteDesc(t, f, ".note.go.buildid")
	return buildIDs{goID: string(goID), gnuID: hex.EncodeToString(noteDesc(t, f, ".note.gnu.build-id"))}
}

func output(t *testing.T, target string, flags ...string) string {
	t.Helper()
	if err := bazel_testing.RunBazel(append([]string{"build", target}, flags...)...); err != nil {
		t.Fatal(err)
	}
	out, err := bazel_testing.BazelOutput(append([]string{"cquery", "--output=files", target}, flags...)...)
	if err != nil {
		t.Fatal(err)
	}
	return strings.TrimSpace(string(out))
}

func loadCommand(f *macho.File, cmd uint32) []byte {
	for _, l := range f.Loads {
		if raw := l.Raw(); f.ByteOrder.Uint32(raw) == cmd {
			return raw
		}
	}
	return nil
}

func noteDesc(t *testing.T, f *elf.File, name string) []byte {
	t.Helper()
	s := f.Section(name)
	if s == nil {
		return nil
	}
	data, err := s.Data()
	if err != nil {
		t.Fatal(err)
	}
	namesz := f.ByteOrder.Uint32(data[0:])
	descsz := f.ByteOrder.Uint32(data[4:])
	start := 12 + (namesz+3)&^3
	return bytes.Clone(data[start : start+descsz])
}
