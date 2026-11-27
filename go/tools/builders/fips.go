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
	"strings"
)

// fipsSnapshot reports whether GOFIPS140 selects a snapshot in lib/fips140,
// as opposed to "off" or "latest", which builds from the SDK's own sources.
func fipsSnapshot() bool {
	v := os.Getenv("GOFIPS140")
	return v != "" && v != "off" && v != "latest"
}

// fipsGorootPaths returns the GOROOT paths the go command needs in addition
// to the usual ones to build with the current GOFIPS140.
func fipsGorootPaths() []string {
	if fipsSnapshot() {
		return []string{"lib/fips140"}
	}
	return nil
}

// readFIPSPackageList returns the import paths of the GOFIPS140 snapshot
// packages listed in the file at path, one per line.
func readFIPSPackageList(path string) ([]string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return strings.Fields(string(data)), nil
}
