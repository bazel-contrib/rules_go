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
	"sort"
	"strings"
	"testing"

	"github.com/bazelbuild/rules_go/go/runfiles"
)

// fixtureZipEntries mimics the layout of a real GOFIPS140 snapshot zip.
func fixtureZipEntries(resolved string) []string {
	base := "golang.org/fips140@" + resolved
	return []string{
		base + "/fips140/v1.0.0/sha256/sha256.go",
		base + "/fips140/v1.0.0/aes/gcm/gcm.go",            // multi-level package
		base + "/fips140/v1.0.0/sha256/sha256_test.go",     // _test.go must be filtered
		base + "/fips140/v1.0.0/testonly/testonly_test.go", // test-only dir: yields no package
		base + "/fips140/v1.0.0/LICENSE",                   // non-.go must be ignored
		base + "/doc.go",                                   // root .go (no /fips140/ marker): ignored
		base + "/go.mod",                                   // no /fips140/ marker: ignored
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

// buildFixture writes a snapshot zip and its alias file under a temp lib dir and
// returns the dir along with the package set both enumerations must produce.
func buildFixture(t *testing.T) (libDir string, want []string) {
	t.Helper()
	libDir = t.TempDir()
	const resolved = "v1.0.0-c2097c7c"
	writeFixtureZip(t, filepath.Join(libDir, resolved+".zip"), fixtureZipEntries(resolved))
	// Alias file: GOFIPS140=v1.0.0 resolves to the pinned snapshot version.
	if err := os.WriteFile(filepath.Join(libDir, "v1.0.0.txt"), []byte(resolved+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	return libDir, []string{
		"crypto/internal/fips140/v1.0.0/aes/gcm",
		"crypto/internal/fips140/v1.0.0/sha256",
	}
}

func TestFIPSSnapshotPackages(t *testing.T) {
	libDir, want := buildFixture(t)
	got, err := fipsSnapshotPackages(libDir, "v1.0.0")
	if err != nil {
		t.Fatalf("fipsSnapshotPackages: %v", err)
	}
	sort.Strings(got)
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Fatalf("fipsSnapshotPackages mismatch:\n got: %v\nwant: %v", got, want)
	}
}

// TestFIPSSnapshotPackagesMatchScript pins fipsSnapshotPackages to the shell
// script that builds packages.txt for the linker: both must enumerate the same
// packages from the same zip, or the importcfg and the on-disk archives diverge.
// Skipped when the script or the tools it relies on are unavailable (e.g.
// minimal/non-Linux environments, or `go test` without Bazel runfiles).
func TestFIPSSnapshotPackagesMatchScript(t *testing.T) {
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

	libDir, want := buildFixture(t)
	dir := t.TempDir()
	// An empty "normal" list makes the script's output exactly the FIPS packages.
	normal := filepath.Join(dir, "normal.txt")
	if err := os.WriteFile(normal, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(dir, "packages.txt")

	cmd := exec.Command("bash", script, normal, out, libDir, "v1.0.0")
	if b, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("running %s: %v\n%s", script, err, b)
	}
	data, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, l := range strings.Split(string(data), "\n") {
		if l = strings.TrimSpace(l); l != "" {
			got = append(got, l)
		}
	}
	sort.Strings(got)
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Fatalf("shell script enumeration differs from fipsSnapshotPackages:\n script: %v\n     go: %v", got, want)
	}
}
