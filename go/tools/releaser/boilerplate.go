// Copyright 2021 The Bazel Authors. All rights reserved.
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
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strings"

	"golang.org/x/mod/semver"
)

func genBoilerplate(version, shasum, goVersion, rnotesData string) string {
	trimmedVersion := strings.TrimPrefix(version, "v")
	return fmt.Sprintf(`%[4]s

## `+"`MODULE.bazel`"+` code

`+"```"+`
bazel_dep(name = "rules_go", version = "%[1]s")

go_sdk = use_extension("@rules_go//go:extensions.bzl", "go_sdk")
go_sdk.download(version = "%[3]s")
`+"```"+`

## `+"`WORKSPACE`"+` code

`+"```"+`
load("@bazel_tools//tools/build_defs/repo:http.bzl", "http_archive")

http_archive(
    name = "io_bazel_rules_go",
    sha256 = "%[2]s",
    urls = [
        "https://mirror.bazel.build/github.com/bazel-contrib/rules_go/releases/download/v%[1]s/rules_go-v%[1]s.zip",
        "https://github.com/bazel-contrib/rules_go/releases/download/v%[1]s/rules_go-v%[1]s.zip",
    ],
)

load("@io_bazel_rules_go//go:deps.bzl", "go_register_toolchains", "go_rules_dependencies")

go_rules_dependencies()

go_register_toolchains(version = "%[3]s")
`+"```\n", trimmedVersion, shasum, goVersion, rnotesData)
}

func findLatestGoVersion() (v string, err error) {
	defer func() {
		if err != nil {
			err = fmt.Errorf("finding latest go version: %w", err)
		}
	}()
	resp, err := http.Get("https://golang.org/dl/?mode=json")
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	type version struct {
		Version string
	}
	var versions []version
	if err := json.Unmarshal(data, &versions); err != nil {
		return "", err
	}
	if len(versions) == 0 {
		return "", errors.New("no versions found")
	}
	sort.Slice(versions, func(i, j int) bool {
		vi := "v" + versions[i].Version[len("go"):]
		vj := "v" + versions[j].Version[len("go"):]
		return semver.Compare(vi, vj) > 0
	})
	return versions[0].Version[len("go"):], nil
}
