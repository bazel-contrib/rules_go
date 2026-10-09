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
	"bytes"
	"crypto/sha256"
	"debug/elf"
	"debug/macho"
	"encoding/hex"
	"fmt"
	"os"
	"strings"
)

// buildIDPlaceholder makes the linker emit the Go and host build IDs, which
// setContentBuildID overwrites in place once the binary is linked. Its length
// matches the hex-encoded ID written over it.
var buildIDPlaceholder = strings.Repeat("0", 2*buildIDSize)

const buildIDSize = 20

// ELF note sections holding the Go and GNU build IDs, as written by cmd/link:
// https://cs.opensource.google/go/go/+/refs/tags/go1.26.7:src/cmd/link/internal/ld/elf.go;l=938-963
const (
	elfGoBuildIDSection  = ".note.go.buildid"
	elfGNUBuildIDSection = ".note.gnu.build-id"
)

// Delimiters of the quoted Go build ID that cmd/link stores in the text segment
// of non-ELF binaries, as defined in cmd/internal/buildid:
// https://cs.opensource.google/go/go/+/refs/tags/go1.26.7:src/cmd/internal/buildid/buildid.go;l=241-242
const (
	goBuildPrefix = "\xff Go build ID: \""
	goBuildEnd    = "\"\n \xff"
)

// hasContentBuildID reports whether setContentBuildID supports binaries for goos.
func hasContentBuildID(goos string) bool {
	switch goos {
	case "android", "darwin", "dragonfly", "freebsd", "illumos", "ios", "linux", "netbsd", "openbsd", "solaris":
		return true
	}
	return false
}

// setContentBuildID replaces the placeholder Go build ID and the host build ID
// (ELF GNU build ID or Mach-O LC_UUID) of the binary at path with a hash of the
// binary, so that distinct binaries get distinct IDs and rebuilds stay
// reproducible. Binaries without the placeholder, e.g. built with -buildid in
// gc_linkopts, are left untouched.
func setContentBuildID(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	sum := sha256.Sum256(data)
	if bytes.HasPrefix(data, []byte(elf.ELFMAG)) {
		return setELFBuildID(path, data, sum)
	}
	return setMachOBuildID(path, data, sum)
}

func setELFBuildID(path string, data []byte, sum [sha256.Size]byte) error {
	f, err := elf.NewFile(bytes.NewReader(data))
	if err != nil {
		return err
	}
	goStart, goEnd, ok := noteDesc(f, elfGoBuildIDSection)
	if !ok || string(data[goStart:goEnd]) != buildIDPlaceholder {
		return nil
	}

	patches := map[int64][]byte{goStart: []byte(hex.EncodeToString(sum[:buildIDSize]))}
	// The GNU note is missing when the external linker does not support --build-id.
	if start, end, ok := noteDesc(f, elfGNUBuildIDSection); ok {
		desc := make([]byte, end-start)
		copy(desc, sum[:])
		patches[start] = desc
	}
	return patchFile(path, patches)
}

// setMachOBuildID re-signs the binary after patching it, because the code
// signature covers both the Go build ID and LC_UUID.
func setMachOBuildID(path string, data []byte, sum [sha256.Size]byte) error {
	f, err := macho.NewFile(bytes.NewReader(data))
	if err != nil {
		return err
	}
	if f.Magic != macho.Magic64 {
		return fmt.Errorf("not 64-bit Mach-O file: %s", path)
	}
	// The Go linker stores the Go build ID in the text segment on Mach-O.
	i := bytes.Index(data, []byte(goBuildPrefix+buildIDPlaceholder+goBuildEnd))
	if i < 0 {
		return nil
	}

	patches := map[int64][]byte{int64(i + len(goBuildPrefix)): []byte(hex.EncodeToString(sum[:buildIDSize]))}
	if off, ok := machoUUIDOffset(f); ok {
		uuid := make([]byte, 16)
		copy(uuid, sum[:])
		// Same RFC 4122 version and variant bits as the Go linker sets.
		uuid[6] = uuid[6]&0x0f | 0x30
		uuid[8] = uuid[8]&0x3f | 0xc0
		patches[off] = uuid
	}
	if err := patchFile(path, patches); err != nil {
		return err
	}
	return machoCodeSign(path)
}

// patchFile overwrites the file at path with each patch at its offset.
func patchFile(path string, patches map[int64][]byte) error {
	out, err := os.OpenFile(path, os.O_WRONLY, 0)
	if err != nil {
		return err
	}
	defer out.Close()
	for off, b := range patches {
		if _, err := out.WriteAt(b, off); err != nil {
			return err
		}
	}
	return out.Close()
}

// machoUUIDOffset returns the file offset of the LC_UUID payload.
func machoUUIDOffset(f *macho.File) (int64, bool) {
	const lcUUID = 0x1b
	off := int64(machoHeaderSize64)
	for _, l := range f.Loads {
		raw := l.Raw()
		if f.ByteOrder.Uint32(raw) == lcUUID {
			return off + 8, true
		}
		off += int64(f.ByteOrder.Uint32(raw[4:]))
	}
	return 0, false
}

// noteDesc returns the file offsets of the descriptor of the first note in the
// named section.
func noteDesc(f *elf.File, name string) (start, end int64, ok bool) {
	s := f.Section(name)
	if s == nil || s.Type != elf.SHT_NOTE {
		return 0, 0, false
	}
	var hdr [12]byte
	if _, err := s.ReadAt(hdr[:], 0); err != nil {
		return 0, 0, false
	}
	namesz := int64(f.ByteOrder.Uint32(hdr[0:]))
	descsz := int64(f.ByteOrder.Uint32(hdr[4:]))
	start = int64(len(hdr)) + (namesz+3)&^3
	if start+descsz > int64(s.Size) {
		return 0, 0, false
	}
	start += int64(s.Offset)
	return start, start + descsz, true
}
