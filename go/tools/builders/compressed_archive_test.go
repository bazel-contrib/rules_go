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
	"io"
	"os"
	"path/filepath"
	"testing"
)

func TestCompressedArchive(t *testing.T) {
	want := bytes.Repeat([]byte("!<arch>\npackage object data\n"), 1000)
	dir := t.TempDir()
	var previous []byte
	for _, name := range []string{"first.a", "second.a"} {
		filename := filepath.Join(dir, name)
		if err := os.WriteFile(filename, want, 0o644); err != nil {
			t.Fatal(err)
		}
		if err := compressArchive(filename); err != nil {
			t.Fatal(err)
		}
		compressed, err := os.ReadFile(filename)
		if err != nil {
			t.Fatal(err)
		}
		if previous != nil && !bytes.Equal(previous, compressed) {
			t.Fatal("compressed bytes depend on filename or other metadata")
		}
		previous = compressed
		expanded, err := expandArchive(filename, dir)
		if err != nil {
			t.Fatal(err)
		}
		got, err := os.ReadFile(expanded)
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(got, want) {
			t.Fatal("expanded archive differs from original")
		}
		unchanged, err := os.ReadFile(filename)
		if err != nil || !bytes.Equal(unchanged, compressed) {
			t.Fatalf("expansion modified input: %v", err)
		}
	}
}

func TestExpandRawAndCorruptArchives(t *testing.T) {
	for _, tc := range []struct {
		name    string
		data    []byte
		wantErr error
	}{
		{name: "raw", data: []byte("!<arch>\nraw archive")},
		{name: "truncated header", data: []byte{0x1f, 0x8b}, wantErr: io.ErrUnexpectedEOF},
		{name: "truncated payload", data: []byte{0x1f, 0x8b, 8, 0, 0, 0, 0, 0, 0, 255, 3}, wantErr: io.ErrUnexpectedEOF},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			filename := filepath.Join(dir, "input.a")
			if err := os.WriteFile(filename, tc.data, 0o644); err != nil {
				t.Fatal(err)
			}
			got, err := expandArchive(filename, dir)
			if tc.wantErr != nil {
				if !errors.Is(err, tc.wantErr) {
					t.Fatalf("got %v, want %v", err, tc.wantErr)
				}
				entries, err := os.ReadDir(dir)
				if err != nil || len(entries) != 1 {
					t.Fatalf("failed expansion left scratch output: %v, %v", entries, err)
				}
				return
			}
			if err != nil || got != filename {
				t.Fatalf("raw archive not preserved: got %q, err %v", got, err)
			}
		})
	}
}
