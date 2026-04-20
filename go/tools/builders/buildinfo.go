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
	"runtime/debug"
	"sort"
	"strconv"
	"strings"
)

const (
	// Match cmd/go's modinfo framing markers:
	// https://go.dev/src/cmd/go/internal/modload/build.go#L29-L30
	buildInfoStart = "\x30\x77\xaf\x0c\x92\x74\x08\x02\x41\xe1\xc1\x07\xe6\xd6\x18\xe6"
	buildInfoEnd   = "\xf9\x32\x43\x31\x86\x18\x20\x72\x00\x82\x42\x10\x41\x16\xd8\xf2"
)

type moduleInfo struct {
	path    string
	version string
}

func buildInfoDeps(modules []moduleInfo) []*debug.Module {
	seen := map[moduleInfo]struct{}{}
	unique := make([]moduleInfo, 0, len(modules))
	for _, module := range modules {
		if module.path == "" || module.version == "" {
			continue
		}
		if _, ok := seen[module]; ok {
			continue
		}
		seen[module] = struct{}{}
		unique = append(unique, module)
	}

	sort.Slice(unique, func(i, j int) bool {
		if unique[i].path != unique[j].path {
			return unique[i].path < unique[j].path
		}
		return unique[i].version < unique[j].version
	})

	deps := make([]*debug.Module, 0, len(unique))
	for _, module := range unique {
		deps = append(deps, &debug.Module{
			Path:    module.path,
			Version: module.version,
		})
	}
	return deps
}

func modInfoData(modules []moduleInfo) string {
	deps := buildInfoDeps(modules)
	if len(deps) == 0 {
		return buildInfoStart + buildInfoEnd
	}

	// debug.BuildInfo.String was added after Go 1.17. Emit the dep-only
	// modinfo format directly so older SDK builders still compile.
	var buf strings.Builder
	for _, dep := range deps {
		buf.WriteString("dep\t")
		buf.WriteString(dep.Path)
		buf.WriteByte('\t')
		buf.WriteString(dep.Version)
		buf.WriteString("\t\n")
	}
	return buildInfoStart + buf.String() + buildInfoEnd
}

func shouldEmitBuildInfo(goVersion, buildmode string) bool {
	switch buildmode {
	case "c-archive", "c-shared", "plugin":
		return false
	default:
		return supportsBuildInfo(goVersion)
	}
}

func supportsBuildInfo(goVersion string) bool {
	if goVersion == "" {
		return true
	}
	goVersion = strings.TrimPrefix(goVersion, "go")
	parts := strings.SplitN(goVersion, ".", 3)
	if len(parts) != 2 {
		if len(parts) == 3 {
			parts = parts[:2]
		} else {
			// Keep build info enabled for newer or non-standard version strings
			// we don't recognize.
			return true
		}
	}
	minor := parts[1]
	for i := 0; i < len(minor); i++ {
		if minor[i] < '0' || minor[i] > '9' {
			minor = minor[:i]
			break
		}
	}
	if minor == "" {
		return true
	}
	major, err := strconv.Atoi(parts[0])
	if err != nil {
		return true
	}
	minorVersion, err := strconv.Atoi(minor)
	if err != nil {
		return true
	}
	return major > 1 || (major == 1 && minorVersion >= 18)
}
