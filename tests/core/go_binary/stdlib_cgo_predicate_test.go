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

// Package stdlib_cgo_predicate_test compares stdlib_uses_cgo in
// go/private/actions/link.bzl with "go list" for the Go SDK of this
// repository. It fails when a Go release adds, removes, or changes a standard
// library package that links cgo code.
package stdlib_cgo_predicate_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"testing"

	"github.com/bazelbuild/rules_go/go/runfiles"
)

// matrixRow is one stdlib_uses_cgo result, written by stdlib_cgo_predicate.bzl.
type matrixRow struct {
	GOOS    string   `json:"goos"`
	GOARCH  string   `json:"goarch"`
	Tags    []string `json:"tags"`
	UsesCgo bool     `json:"uses_cgo"`
}

// listedPackage holds the fields of "go list -json" that the test reads.
type listedPackage struct {
	ImportPath string
	CgoFiles   []string
	Imports    []string
}

// excluded holds the standard library packages with cgo files that
// stdlib_uses_cgo does not model, with the reason for each one.
//
// The test also skips test-only packages: internal packages that no importable
// standard library package imports, for example internal/testpty. A binary
// cannot import them.
var excluded = map[string]string{
	// The test checks below that only packages with cgo files import
	// runtime/cgo. stdlib_uses_cgo models those packages.
	"runtime/cgo": "linked only through other cgo packages",
	// Analysis cannot see which packages a binary imports. go/modes.rst tells
	// users to set pure = "off" for a binary that imports plugin.
	"plugin": "documented limitation",
	// These packages build only with the race, msan, or asan tag. GoLink never
	// links internally in race or msan mode, and stdlib_uses_cgo treats the
	// asan tag as cgo.
	"runtime/race": "race mode only",
	"runtime/msan": "msan mode only",
	"runtime/asan": "asan tag only",
}

func TestStdlibCgoPredicate(t *testing.T) {
	goBin := rlocation(t, "GO")
	rows := readMatrix(t, rlocation(t, "MATRIX"))

	// Run "go list" outside of any module, with a cache in the test's
	// temporary directory.
	tmp := t.TempDir()
	modDir := filepath.Join(tmp, "mod")
	if err := os.MkdirAll(modDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(modDir, "go.mod"), []byte("module stdlibcgopredicate\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	env := append(os.Environ(),
		"CGO_ENABLED=1",
		"GOCACHE="+filepath.Join(tmp, "gocache"),
		"GOPATH="+filepath.Join(tmp, "gopath"),
		"GOENV=off",
		"GOFLAGS=",
		"GOTOOLCHAIN=local",
		"GOWORK=off",
	)

	supported := supportedPlatforms(t, goBin, modDir, env)
	byPlatform := make(map[string][]matrixRow)
	for _, row := range rows {
		platform := row.GOOS + "/" + row.GOARCH
		byPlatform[platform] = append(byPlatform[platform], row)
	}

	// Check every GOOS with one GOARCH, the first one in platforms.bzl that
	// the SDK supports. Also check linux/amd64 and linux/arm64, the only
	// platforms where BoringCrypto uses cgo.
	var platforms []string
	seenGOOS := make(map[string]bool)
	for _, row := range rows {
		platform := row.GOOS + "/" + row.GOARCH
		if seenGOOS[row.GOOS] || !supported[platform] {
			continue
		}
		seenGOOS[row.GOOS] = true
		platforms = append(platforms, platform)
	}
	for _, platform := range []string{"linux/amd64", "linux/arm64"} {
		if !supported[platform] {
			t.Fatalf("go tool dist list does not report %s", platform)
		}
		if !slices.Contains(platforms, platform) {
			platforms = append(platforms, platform)
		}
	}
	t.Logf("checking %s", strings.Join(platforms, " "))

	for _, platform := range platforms {
		t.Run(strings.ReplaceAll(platform, "/", "_"), func(t *testing.T) {
			t.Parallel()
			goos, goarch, _ := strings.Cut(platform, "/")
			platformEnv := append(slices.Clip(env), "GOOS="+goos, "GOARCH="+goarch)
			for _, row := range byPlatform[platform] {
				remaining, err := stdlibCgoPackages(goBin, modDir, platformEnv, row.Tags)
				if err != nil {
					t.Errorf("tags %q: %v", row.Tags, err)
					continue
				}
				if row.UsesCgo != (len(remaining) > 0) {
					t.Errorf("tags %q: stdlib_uses_cgo returns %v, but go list reports cgo files in %q", row.Tags, row.UsesCgo, remaining)
				}
			}
		})
	}
}

// stdlibCgoPackages returns the standard library packages that have cgo files
// for the platform in env, and that a binary can link. Packages in excluded
// are not returned.
func stdlibCgoPackages(goBin, dir string, env, tags []string) ([]string, error) {
	cmd := exec.Command(goBin, "list", "-e", "-tags="+strings.Join(tags, ","), "-json=ImportPath,CgoFiles,Imports", "std")
	cmd.Dir = dir
	cmd.Env = env
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("go list: %v: %s", err, stderr.Bytes())
	}

	pkgs := make(map[string]listedPackage)
	dec := json.NewDecoder(bytes.NewReader(out))
	for {
		var pkg listedPackage
		if err := dec.Decode(&pkg); err == io.EOF {
			break
		} else if err != nil {
			return nil, err
		}
		pkgs[pkg.ImportPath] = pkg
	}

	// Find the packages that a binary can link: the importable packages and
	// the packages that they import.
	var queue []string
	for path := range pkgs {
		if importable(path) {
			queue = append(queue, path)
		}
	}
	reachable := make(map[string]bool)
	for len(queue) > 0 {
		path := queue[len(queue)-1]
		queue = queue[:len(queue)-1]
		if reachable[path] {
			continue
		}
		reachable[path] = true
		for _, imp := range pkgs[path].Imports {
			if _, ok := pkgs[imp]; ok && !reachable[imp] {
				queue = append(queue, imp)
			}
		}
	}

	var remaining []string
	for path := range reachable {
		pkg := pkgs[path]
		if len(pkg.CgoFiles) == 0 {
			if slices.Contains(pkg.Imports, "runtime/cgo") {
				return nil, fmt.Errorf("%s imports runtime/cgo without cgo files, so stdlib_uses_cgo must model it", path)
			}
			continue
		}
		if _, ok := excluded[path]; !ok {
			remaining = append(remaining, path)
		}
	}
	sort.Strings(remaining)
	return remaining, nil
}

// importable reports whether code outside of the standard library can import
// the package.
func importable(path string) bool {
	if strings.HasPrefix(path, "vendor/") {
		return false
	}
	return !slices.Contains(strings.Split(path, "/"), "internal")
}

// supportedPlatforms returns the GOOS/GOARCH pairs that the Go SDK supports.
func supportedPlatforms(t *testing.T, goBin, dir string, env []string) map[string]bool {
	t.Helper()
	cmd := exec.Command(goBin, "tool", "dist", "list")
	cmd.Dir = dir
	cmd.Env = env
	cmd.Stderr = os.Stderr
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("go tool dist list: %v", err)
	}
	supported := make(map[string]bool)
	for _, line := range strings.Fields(string(out)) {
		supported[line] = true
	}
	return supported
}

func readMatrix(t *testing.T, path string) []matrixRow {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var rows []matrixRow
	if err := json.Unmarshal(data, &rows); err != nil {
		t.Fatal(err)
	}
	if len(rows) == 0 {
		t.Fatal("the stdlib_uses_cgo matrix is empty")
	}
	return rows
}

// rlocation returns the path of the runfile named by an environment variable.
func rlocation(t *testing.T, envVar string) string {
	t.Helper()
	path, err := runfiles.Rlocation(os.Getenv(envVar))
	if err != nil {
		t.Fatalf("%s: %v", envVar, err)
	}
	return path
}
