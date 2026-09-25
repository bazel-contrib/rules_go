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
	"encoding/hex"
	"os"
	"strings"
)

// buildIDPlaceholder makes the linker emit the Go and GNU build ID notes, which
// setContentBuildID overwrites in place once the binary is linked. Its length
// matches the hex-encoded ID written over it.
var buildIDPlaceholder = strings.Repeat("0", 2*buildIDSize)

const buildIDSize = 20

// hasContentBuildID reports whether setContentBuildID supports binaries for goos.
func hasContentBuildID(goos string) bool {
	switch goos {
	case "android", "dragonfly", "freebsd", "illumos", "linux", "netbsd", "openbsd", "solaris":
		return true
	}
	return false
}

// setContentBuildID replaces the placeholder Go build ID and the GNU build ID
// of the ELF binary at path with a hash of the binary, so that distinct
// binaries get distinct IDs and rebuilds stay reproducible. Binaries without
// the placeholder, e.g. built with -buildid in gc_linkopts, are left untouched.
func setContentBuildID(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	f, err := elf.NewFile(bytes.NewReader(data))
	if err != nil {
		return err
	}
	goStart, goEnd, ok := noteDesc(f, ".note.go.buildid")
	if !ok || string(data[goStart:goEnd]) != buildIDPlaceholder {
		return nil
	}

	sum := sha256.Sum256(data)
	out, err := os.OpenFile(path, os.O_WRONLY, 0)
	if err != nil {
		return err
	}
	defer out.Close()
	if _, err := out.WriteAt([]byte(hex.EncodeToString(sum[:buildIDSize])), goStart); err != nil {
		return err
	}
	// The GNU note is missing when the external linker does not support --build-id.
	if start, end, ok := noteDesc(f, ".note.gnu.build-id"); ok {
		desc := make([]byte, end-start)
		copy(desc, sum[:])
		if _, err := out.WriteAt(desc, start); err != nil {
			return err
		}
	}
	return out.Close()
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
