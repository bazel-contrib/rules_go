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
	"bufio"
	"os"
	"strings"
)

// fipsPackagesFromList reads the versioned GOFIPS140 snapshot import paths from
// fipsPackageListPath, one per line (blank lines ignored). That file is produced
// by the package-list step (generate_fips_package_list.sh) — the single
// enumeration of the snapshot, also reflected in packages.txt for the linker's
// importcfg — so the builder consumes it verbatim rather than re-deriving the set
// from the snapshot zip.
func fipsPackagesFromList(fipsPackageListPath string) ([]string, error) {
	f, err := os.Open(fipsPackageListPath)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var pkgs []string
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		if line := strings.TrimSpace(scanner.Text()); line != "" {
			pkgs = append(pkgs, line)
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return pkgs, nil
}
