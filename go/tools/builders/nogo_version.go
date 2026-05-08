/* Copyright 2026 The Bazel Authors. All rights reserved.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

   http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package main

import (
	"regexp"
	"strings"
)

// Match the longest Go version prefix that go/types and x/tools analyzers
// understand: an optional "go" prefix, 1-3 numeric components, and optional
// beta/rc prerelease suffixes. This intentionally stops before custom toolchain
// metadata such as "-abcdef" in "go1.26-abcdef".
var supportedGoVersionPrefix = regexp.MustCompile(`^(go)?[0-9]+(\.[0-9]+){0,2}((beta|rc)[0-9]+)?`)

func normalizeGoVersion(goVersion string) string {
	if goVersion == "" {
		return ""
	}
	if !strings.HasPrefix(goVersion, "go") {
		goVersion = "go" + goVersion
	}
	if prefix := supportedGoVersionPrefix.FindString(goVersion); prefix != "" {
		goVersion = prefix
	}
	return normalizeGoVersionForTypes(goVersion)
}

func trimGoPatchVersion(goVersion string) string {
	prefix := ""
	if strings.HasPrefix(goVersion, "go") {
		prefix = "go"
		goVersion = strings.TrimPrefix(goVersion, "go")
	}
	dot := strings.IndexByte(goVersion, '.')
	if dot < 0 {
		return prefix + goVersion
	}
	major, minor := goVersion[:dot], goVersion[dot+1:]
	end := 0
	for end < len(minor) && minor[end] >= '0' && minor[end] <= '9' {
		end++
	}
	if end == 0 {
		return prefix + goVersion
	}
	return prefix + major + "." + minor[:end]
}
