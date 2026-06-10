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
	"path/filepath"
	"strings"
)

// fipsSnapshotPackages enumerates the import paths contributed by the GOFIPS140
// snapshot. The version resolves from lib/fips140/<gofips140>.txt when that is an
// alias (e.g. v1.0.0 -> v1.0.0-c2097c7c), then the matching zip is read. Entries
// look like golang.org/fips140@<ver>/fips140/<v>/<pkg>/<file>.go and map to the
// import path crypto/internal/fips140/<v>/<pkg>.
//
// IMPORTANT: this MUST produce the same package set as
// go/private/rules/generate_fips_package_list.sh, which performs the same
// enumeration to build packages.txt for the linker's importcfg. If the two
// diverge, the importcfg (from packages.txt) and the on-disk archives (built
// here) will mismatch at link time. fips_test.go pins the two together.
func fipsSnapshotPackages(libDir, gofips140 string) ([]string, error) {
	version := gofips140
	if data, err := os.ReadFile(filepath.Join(libDir, gofips140+".txt")); err == nil {
		version = strings.TrimSpace(string(data))
	}
	r, err := zip.OpenReader(filepath.Join(libDir, version+".zip"))
	if err != nil {
		return nil, err
	}
	defer r.Close()

	const marker = "/fips140/"
	set := map[string]bool{}
	for _, f := range r.File {
		name := f.Name
		if !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		i := strings.Index(name, marker)
		if i < 0 {
			continue
		}
		// rel is "<v>/<pkg>/<file>.go"; drop the trailing file to get the package.
		rel := name[i+len(marker):]
		j := strings.LastIndex(rel, "/")
		if j < 0 {
			continue
		}
		set["crypto/internal/fips140/"+rel[:j]] = true
	}
	pkgs := make([]string, 0, len(set))
	for p := range set {
		pkgs = append(pkgs, p)
	}
	return pkgs, nil
}
