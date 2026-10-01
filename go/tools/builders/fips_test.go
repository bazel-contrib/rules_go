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
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestFIPSPackagesFromList(t *testing.T) {
	// The dedicated FIPS list has one versioned snapshot package per line with a
	// trailing newline, matching the package-list rule's sorted output.
	path := filepath.Join(t.TempDir(), "fips_packages.txt")
	contents := `crypto/internal/fips140/v1.0.0-c2097c7c
crypto/internal/fips140/v1.0.0-c2097c7c/aes
crypto/internal/fips140/v1.0.0-c2097c7c/aes/gcm
crypto/internal/fips140/v1.0.0-c2097c7c/sha256
`
	if err := os.WriteFile(path, []byte(contents), 0o644); err != nil {
		t.Fatal(err)
	}

	got, err := fipsPackagesFromList(path)
	if err != nil {
		t.Fatalf("fipsPackagesFromList: %v", err)
	}
	want := []string{
		"crypto/internal/fips140/v1.0.0-c2097c7c",
		"crypto/internal/fips140/v1.0.0-c2097c7c/aes",
		"crypto/internal/fips140/v1.0.0-c2097c7c/aes/gcm",
		"crypto/internal/fips140/v1.0.0-c2097c7c/sha256",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("fipsPackagesFromList mismatch:\n got: %v\nwant: %v", got, want)
	}
}

func TestFIPSPackagesFromListIgnoresBlankLines(t *testing.T) {
	path := filepath.Join(t.TempDir(), "fips_packages.txt")
	if err := os.WriteFile(path, []byte("\n\ncrypto/internal/fips140/v1.0.0/aes\n\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	got, err := fipsPackagesFromList(path)
	if err != nil {
		t.Fatalf("fipsPackagesFromList: %v", err)
	}
	if want := []string{"crypto/internal/fips140/v1.0.0/aes"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("fipsPackagesFromList mismatch:\n got: %v\nwant: %v", got, want)
	}
}

// A non-FIPS SDK still writes fips_packages.txt, empty, so the predeclared
// output always exists; the builder must read that as "no snapshot packages"
// rather than failing.
func TestFIPSPackagesFromListEmptyFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "fips_packages.txt")
	if err := os.WriteFile(path, nil, 0o644); err != nil {
		t.Fatal(err)
	}

	got, err := fipsPackagesFromList(path)
	if err != nil {
		t.Fatalf("fipsPackagesFromList: %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("fipsPackagesFromList = %v, want empty", got)
	}
}
