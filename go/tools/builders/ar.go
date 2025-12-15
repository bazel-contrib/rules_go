// Copyright 2018 The Bazel Authors. All rights reserved.
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
	"encoding/binary"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
)

const (
	// arHeader appears at the beginning of archives created by "ar" and
	// "go tool pack" on all platforms.
	arHeader = "!<arch>\n"

	// entryLength is the size in bytes of the metadata preceding each file
	// in an archive.
	entryLength = 60
)

var zeroBytes = []byte("0                    ")

type header struct {
	NameRaw     [16]byte
	ModTimeRaw  [12]byte
	OwnerIdRaw  [6]byte
	GroupIdRaw  [6]byte
	FileModeRaw [8]byte
	FileSizeRaw [10]byte
	EndRaw      [2]byte
}

func (h *header) name() string {
	return strings.TrimRight(string(h.NameRaw[:]), " ")
}

func (h *header) size() int64 {
	s, err := strconv.Atoi(strings.TrimRight(string(h.FileSizeRaw[:]), " "))
	if err != nil {
		panic(err)
	}
	return int64(s)
}

func (h *header) next() int64 {
	size := h.size()
	return size + size%2
}

func (h *header) deterministic() *header {
	h2 := *h
	copy(h2.ModTimeRaw[:], zeroBytes)
	copy(h2.OwnerIdRaw[:], zeroBytes)
	copy(h2.GroupIdRaw[:], zeroBytes)
	copy(h2.FileModeRaw[:], zeroBytes) // GNU ar also clears this
	return &h2
}

// stripArMetadata strips the archive metadata of non-deterministic data:
// - Timestamps
// - User IDs
// - Group IDs
// - File Modes
// The archive is modified in place.
func stripArMetadata(archivePath string) error {
	archive, err := os.OpenFile(archivePath, os.O_RDWR, 0)
	if err != nil {
		return err
	}
	defer archive.Close()

	magic := make([]byte, len(arHeader))
	if _, err := io.ReadFull(archive, magic); err != nil {
		return err
	}

	if string(magic) != arHeader {
		return fmt.Errorf("%s is not an archive", archivePath)
	}

	for {
		hdr := &header{}
		if err := binary.Read(archive, binary.BigEndian, hdr); err == io.EOF {
			return nil
		} else if err != nil {
			return err
		}

		// Seek back at the beginning of the header and overwrite it.
		archive.Seek(-entryLength, os.SEEK_CUR)
		if err := binary.Write(archive, binary.BigEndian, hdr.deterministic()); err != nil {
			return err
		}

		if _, err := archive.Seek(hdr.next(), os.SEEK_CUR); err == io.EOF {
			return nil
		} else if err != nil {
			return err
		}
	}
}
