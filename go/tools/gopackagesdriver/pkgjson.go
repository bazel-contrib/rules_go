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
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"

	"golang.org/x/tools/go/packages"
)

// GOPACKAGESDRIVER_PKG_JSON_LIST is a file of .pkg.json paths (one per line).
// When set, the driver answers from those files and does not invoke Bazel.
var pkgJSONList = os.Getenv("GOPACKAGESDRIVER_PKG_JSON_LIST")

// runFromPkgJSON answers import-path queries from listed .pkg.json files.
// Placeholders resolve against cwd (execroot inside an action).
func runFromPkgJSON(listPath string, request *packages.DriverRequest, queries []string, out io.Writer) error {
	jsonFiles, err := readLines(listPath)
	if err != nil {
		return fmt.Errorf("reading %s: %w", listPath, err)
	}
	root, err := os.Getwd()
	if err != nil {
		return err
	}
	resolve := func(p string) string {
		for _, placeholder := range []string{"__BAZEL_EXECROOT__", "__BAZEL_WORKSPACE__", "__BAZEL_OUTPUT_BASE__"} {
			p = strings.Replace(p, placeholder, root, 1)
		}
		return p
	}
	// bazelVersion is unused for ID lookup on this path (GetResponseFromIDs).
	driver, err := NewJSONPackagesDriver(jsonFiles, resolve, bazelVersion{}, request.Overlay)
	if err != nil {
		return err
	}
	ids, err := driver.registry.MatchImportPaths(queries)
	if err != nil {
		return err
	}
	resp := driver.GetResponseFromIDs(ids)
	return json.NewEncoder(out).Encode(resp)
}

func readLines(path string) ([]string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var lines []string
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		lines = append(lines, line)
	}
	if err := sc.Err(); err != nil {
		return nil, err
	}
	return lines, nil
}
