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
	"archive/zip"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/bazelbuild/rules_go/go/runfiles"
)

func TestFIPSPackagesFromList(t *testing.T) {
	// The dedicated FIPS list has one versioned snapshot package per line; blank
	// lines are ignored and order is preserved.
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

// TestGenerateFIPSPackageListScript is an input/output test for
// generate_fips_package_list.sh: it builds a lightweight lib/fips140 tree (a
// snapshot zip + alias file) mirroring a Go distribution, runs the script, and
// checks both emitted files. Skipped when the script or the shell tools it needs
// are unavailable (e.g. `go test` without Bazel runfiles).
func TestGenerateFIPSPackageListScript(t *testing.T) {
	for _, tool := range []string{"bash", "unzip", "sed", "grep", "sort", "cat"} {
		if _, err := exec.LookPath(tool); err != nil {
			t.Skipf("skipping: %q not on PATH", tool)
		}
	}
	rf, err := runfiles.New()
	if err != nil {
		t.Skipf("skipping: runfiles unavailable: %v", err)
	}
	script, err := rf.Rlocation("io_bazel_rules_go/go/private/rules/generate_fips_package_list.sh")
	if err != nil {
		t.Skipf("skipping: script runfile unavailable: %v", err)
	}
	if _, err := os.Stat(script); err != nil {
		t.Skipf("skipping: script not found: %v", err)
	}

	// Lightweight lib/fips140 tree: alias v1.0.0 -> the resolved version, and a
	// zip whose entries mirror the real distribution layout.
	const resolved = "v1.0.0-c2097c7c"
	libDir := t.TempDir()
	writeFixtureZip(t, filepath.Join(libDir, resolved+".zip"), []string{
		"golang.org/fips140@" + resolved + "/fips140/" + resolved + "/sha256/sha256.go",
		"golang.org/fips140@" + resolved + "/fips140/" + resolved + "/aes/gcm/gcm.go", // multi-level
		"golang.org/fips140@" + resolved + "/fips140/" + resolved + "/sha256/sha256_test.go", // _test.go: filtered
		"golang.org/fips140@" + resolved + "/fips140/" + resolved + "/LICENSE",               // non-.go: ignored
		"golang.org/fips140@" + resolved + "/doc.go",                                         // no /fips140/ marker: ignored
	})
	if err := os.WriteFile(filepath.Join(libDir, "v1.0.0.txt"), []byte(resolved+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	dir := t.TempDir()
	normal := filepath.Join(dir, "normal.txt")
	if err := os.WriteFile(normal, []byte("fmt\nstrings\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(dir, "packages.txt")
	fipsOut := filepath.Join(dir, "fips_packages.txt")

	// args: normal out libdir gofips fips_out
	cmd := exec.Command("bash", script, normal, out, libDir, "v1.0.0", fipsOut)
	if b, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("running %s: %v\n%s", script, err, b)
	}

	wantFips := []string{
		"crypto/internal/fips140/" + resolved + "/aes/gcm",
		"crypto/internal/fips140/" + resolved + "/sha256",
	}
	if got := readLines(t, fipsOut); !reflect.DeepEqual(got, wantFips) {
		t.Fatalf("fips_out mismatch:\n got: %v\nwant: %v", got, wantFips)
	}

	// packages.txt = normal stdlib list + the FIPS packages (sorted, unique).
	wantOut := []string{
		"crypto/internal/fips140/" + resolved + "/aes/gcm",
		"crypto/internal/fips140/" + resolved + "/sha256",
		"fmt",
		"strings",
	}
	if got := readLines(t, out); !reflect.DeepEqual(got, wantOut) {
		t.Fatalf("packages.txt mismatch:\n got: %v\nwant: %v", got, wantOut)
	}
}

func writeFixtureZip(t *testing.T, path string, entries []string) {
	t.Helper()
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	zw := zip.NewWriter(f)
	for _, name := range entries {
		w, err := zw.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := w.Write([]byte("package x\n")); err != nil {
			t.Fatal(err)
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
}

func readLines(t *testing.T, path string) []string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var lines []string
	for _, l := range strings.Split(string(data), "\n") {
		if l = strings.TrimSpace(l); l != "" {
			lines = append(lines, l)
		}
	}
	return lines
}
