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
	"errors"
	"fmt"
	"strconv"
)

// The .x files produced by "go tool compile -pack -o" are ar archives with a
// single __.PKGDEF entry. gcexportdata.NewReader used to unwrap them, but it
// is deprecated and will stop supporting compiler archives in Go 1.29, while
// gcexportdata.Read rejects archives. extractExportData does the unwrapping
// here so that the result can be passed directly to gcexportdata.Read.
//
// The expected layout is:
//
//	!<arch>\n              ar file signature
//	<60-byte ar header>    entry header for __.PKGDEF, including its size
//	go object <...>\n      object header
//	<optional headers>\n   other headers, such as the build ID
//	$$B\n                  binary export data section start
//	<export data>          starts with the format byte ('u' for unified)
//	\n$$\n                 end-of-section marker
//
// See cmd/internal/archive and the "Export" section of cmd/compile/README.

const (
	exportDataArMagic      = "!<arch>\n"
	exportDataArHeaderLen  = 60
	exportDataPkgDef       = "__.PKGDEF"
	exportDataSectionStart = "$$B\n"
	exportDataSectionEnd   = "\n$$\n"
)

// extractExportData returns the export data held in data, which is the
// content of a compiler-produced .x or .a file. If data is not an ar archive,
// it is assumed to already hold bare export data and is returned unchanged.
func extractExportData(data []byte) ([]byte, error) {
	if !bytes.HasPrefix(data, []byte(exportDataArMagic)) {
		return data, nil
	}
	pkgdef, err := findArEntry(data[len(exportDataArMagic):], exportDataPkgDef)
	if err != nil {
		return nil, err
	}
	return stripPkgDefHeaders(pkgdef)
}

// findArEntry returns the content of the archive entry called name. rest is
// the archive content that follows the ar signature.
func findArEntry(rest []byte, name string) ([]byte, error) {
	for len(rest) > 0 {
		if len(rest) < exportDataArHeaderLen {
			return nil, errors.New("truncated archive header")
		}
		hdr := rest[:exportDataArHeaderLen]
		rest = rest[exportDataArHeaderLen:]

		// Header fields: name [0:16], size [48:58], terminator [58:60].
		if string(hdr[58:60]) != "`\n" {
			return nil, errors.New("malformed archive header")
		}
		size, err := strconv.ParseInt(string(bytes.TrimRight(hdr[48:58], " ")), 10, 64)
		if err != nil || size < 0 || size > int64(len(rest)) {
			return nil, fmt.Errorf("invalid archive entry size %q", hdr[48:58])
		}
		// GNU ar ends names with '/'; Go's pack tool pads with spaces.
		entryName := string(bytes.TrimRight(bytes.TrimRight(hdr[0:16], " "), "/"))
		if entryName == name {
			return rest[:size], nil
		}

		// Entries are padded to an even length.
		next := size + size%2
		if next > int64(len(rest)) {
			next = int64(len(rest))
		}
		rest = rest[next:]
	}
	return nil, fmt.Errorf("archive has no %s entry", name)
}

// stripPkgDefHeaders removes the object headers, the section start marker,
// and the end-of-section marker from the content of a __.PKGDEF entry.
func stripPkgDefHeaders(pkgdef []byte) ([]byte, error) {
	if !bytes.HasPrefix(pkgdef, []byte("go object ")) {
		return nil, errors.New("__.PKGDEF does not start with an object header")
	}
	// Skip header lines until the line that starts the data section.
	// Header lines never start with "$$".
	for !bytes.HasPrefix(pkgdef, []byte("$$")) {
		i := bytes.IndexByte(pkgdef, '\n')
		if i < 0 {
			return nil, errors.New("no export data section in __.PKGDEF")
		}
		pkgdef = pkgdef[i+1:]
	}
	if !bytes.HasPrefix(pkgdef, []byte(exportDataSectionStart)) {
		line, _, _ := bytes.Cut(pkgdef, []byte("\n"))
		return nil, fmt.Errorf("unknown export data header %q", line)
	}
	pkgdef = pkgdef[len(exportDataSectionStart):]

	exportData, ok := bytes.CutSuffix(pkgdef, []byte(exportDataSectionEnd))
	if !ok {
		return nil, errors.New("export data has no end-of-section marker")
	}
	return exportData, nil
}
