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
	"encoding/json"
	"go/types"
	"os"
	"path/filepath"
	"testing"

	"golang.org/x/tools/go/packages"
)

func TestRunFromPkgJSON(t *testing.T) {
	dir := t.TempDir()
	writePkgJSON(t, filepath.Join(dir, "dep.pkg.json"), FlatPackage{
		ID:      "//offline/dep:dep",
		Name:    "dep",
		PkgPath: "example.com/offline/dep",
	})
	writePkgJSON(t, filepath.Join(dir, "lib.pkg.json"), FlatPackage{
		ID:      "//offline/lib:lib",
		Name:    "lib",
		PkgPath: "example.com/offline/lib",
		Imports: map[string]string{
			"example.com/offline/dep": "//offline/dep:dep",
		},
	})
	listPath := filepath.Join(dir, "pkg_json_list")
	if err := os.WriteFile(listPath, []byte(
		filepath.Join(dir, "dep.pkg.json")+"\n"+
			filepath.Join(dir, "lib.pkg.json")+"\n",
	), 0o644); err != nil {
		t.Fatal(err)
	}

	oldWD, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(oldWD) })

	var out bytes.Buffer
	if err := runFromPkgJSON(listPath, &packages.DriverRequest{}, []string{"example.com/offline/lib"}, &out); err != nil {
		t.Fatal(err)
	}
	var resp packages.DriverResponse
	if err := json.Unmarshal(out.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if len(resp.Roots) != 1 || resp.Roots[0] != "//offline/lib:lib" {
		t.Fatalf("roots: %v", resp.Roots)
	}
	if findPackageByID(resp.Packages, "//offline/dep:dep") == nil {
		t.Fatalf("dep missing from packages: %+v", resp.Packages)
	}
}

func TestRunFromPkgJSONTargetSizes(t *testing.T) {
	dir := t.TempDir()
	pkgPath := filepath.Join(dir, "lib.pkg.json")
	writePkgJSON(t, pkgPath, FlatPackage{
		ID:      "//lib:lib",
		Name:    "lib",
		PkgPath: "example.com/lib",
	})
	listPath := filepath.Join(dir, "list")
	if err := os.WriteFile(listPath, []byte(pkgPath+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	oldBuildContext := buildContext
	t.Cleanup(func() { buildContext = oldBuildContext })

	for _, tc := range []struct {
		arch     string
		wordSize int64
	}{
		{arch: "386", wordSize: 4},
		{arch: "amd64", wordSize: 8},
		{arch: "arm64", wordSize: 8},
	} {
		t.Run(tc.arch, func(t *testing.T) {
			targetContext := *oldBuildContext
			targetContext.Compiler = "gc"
			targetContext.GOARCH = tc.arch
			buildContext = &targetContext

			var out bytes.Buffer
			request := &packages.DriverRequest{Mode: packages.NeedTypesSizes}
			if err := runFromPkgJSON(listPath, request, []string{"example.com/lib"}, &out); err != nil {
				t.Fatal(err)
			}
			var resp packages.DriverResponse
			if err := json.Unmarshal(out.Bytes(), &resp); err != nil {
				t.Fatal(err)
			}

			if resp.Arch != tc.arch {
				t.Errorf("Arch = %q, want %q", resp.Arch, tc.arch)
			}
			// go/packages uses these fields to size types when loading a package.
			// Missing fields cause it to fall back to the client's architecture.
			sizes := types.SizesFor(resp.Compiler, resp.Arch)
			if sizes == nil {
				t.Fatalf("no type sizes for Compiler=%q, Arch=%q", resp.Compiler, resp.Arch)
			}
			if got := sizes.Sizeof(types.Typ[types.Int]); got != tc.wordSize {
				t.Errorf("sizeof(int) = %d, want %d for %s", got, tc.wordSize, tc.arch)
			}
		})
	}
}

func TestRunFromPkgJSONUnsupportedQuery(t *testing.T) {
	dir := t.TempDir()
	writePkgJSON(t, filepath.Join(dir, "lib.pkg.json"), FlatPackage{
		ID:      "//lib:lib",
		Name:    "lib",
		PkgPath: "example.com/lib",
	})
	listPath := filepath.Join(dir, "list")
	if err := os.WriteFile(listPath, []byte(filepath.Join(dir, "lib.pkg.json")+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	err := runFromPkgJSON(listPath, &packages.DriverRequest{}, []string{"file=lib.go"}, &bytes.Buffer{})
	if err == nil {
		t.Fatal("expected error")
	}
}

func writePkgJSON(t *testing.T, path string, pkg FlatPackage) {
	t.Helper()
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if err := json.NewEncoder(f).Encode(pkg); err != nil {
		t.Fatal(err)
	}
}
