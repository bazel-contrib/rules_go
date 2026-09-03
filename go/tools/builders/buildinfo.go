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
	"flag"
	"fmt"
	"net/url"
	"os"
	"runtime/debug"
	"sort"
	"strings"
	"time"
)

const (
	// Match cmd/go's modinfo framing markers:
	// https://go.dev/src/cmd/go/internal/modload/build.go#L29-L30
	buildInfoStart = "\x30\x77\xaf\x0c\x92\x74\x08\x02\x41\xe1\xc1\x07\xe6\xd6\x18\xe6"
	buildInfoEnd   = "\xf9\x32\x43\x31\x86\x18\x20\x72\x00\x82\x42\x10\x41\x16\xd8\xf2"

	stableVCSKey         = "STABLE_VCS"
	stableVCSRevisionKey = "STABLE_VCS_REVISION"
	stableVCSTimeKey     = "STABLE_VCS_TIME"
	stableVCSModifiedKey = "STABLE_VCS_MODIFIED"
)

type moduleInfo struct {
	path    string
	version string
	sum     string
}

type packageMetadata struct {
	PURL string `json:"purl"`
}

func isUnprefixedSemver(version string) bool {
	coreAndPrerelease, build, hasBuild := strings.Cut(version, "+")
	if hasBuild && !validSemverIdentifiers(build, false) {
		return false
	}
	core, prerelease, hasPrerelease := strings.Cut(coreAndPrerelease, "-")
	if hasPrerelease && !validSemverIdentifiers(prerelease, true) {
		return false
	}
	parts := strings.Split(core, ".")
	if len(parts) == 0 || len(parts) > 3 || (hasBuild || hasPrerelease) && len(parts) != 3 {
		return false
	}
	for _, part := range parts {
		if part == "" || len(part) > 1 && part[0] == '0' {
			return false
		}
		for _, c := range []byte(part) {
			if c < '0' || c > '9' {
				return false
			}
		}
	}
	return true
}

func validSemverIdentifiers(value string, rejectNumericLeadingZero bool) bool {
	for _, identifier := range strings.Split(value, ".") {
		if identifier == "" {
			return false
		}
		numeric := true
		for _, c := range []byte(identifier) {
			if c >= '0' && c <= '9' {
				continue
			}
			numeric = false
			if c != '-' && (c < 'A' || c > 'Z') && (c < 'a' || c > 'z') {
				return false
			}
		}
		if rejectNumericLeadingZero && numeric && len(identifier) > 1 && identifier[0] == '0' {
			return false
		}
	}
	return true
}

func modulesFromPackageMetadataFiles(paths []string) ([]moduleInfo, error) {
	modules := make([]moduleInfo, 0, len(paths))
	for _, path := range paths {
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("reading package metadata %q: %w", path, err)
		}
		var metadata packageMetadata
		if err := json.Unmarshal(data, &metadata); err != nil {
			return nil, fmt.Errorf("parsing package metadata %q: %w", path, err)
		}
		module, ok, err := moduleFromPURL(metadata.PURL)
		if err != nil {
			return nil, fmt.Errorf("parsing package metadata %q: %w", path, err)
		}
		if ok {
			modules = append(modules, module)
		}
	}
	return modules, nil
}

func moduleFromPURL(purl string) (moduleInfo, bool, error) {
	u, err := url.Parse(purl)
	if err != nil {
		return moduleInfo{}, false, fmt.Errorf("parsing Go package URL: %w", err)
	}
	value, ok := strings.CutPrefix(u.Opaque, "golang/")
	if u.Scheme != "pkg" || !ok {
		return moduleInfo{}, false, nil
	}
	if value == "" {
		return moduleInfo{}, false, fmt.Errorf("Go package URL has an empty module path")
	}

	modulePath := value
	moduleVersion := "(devel)"
	if i := strings.LastIndexByte(value, '@'); i >= 0 {
		modulePath = value[:i]
		moduleVersion = value[i+1:]
		if moduleVersion == "" {
			moduleVersion = "(devel)"
		}
	}
	if modulePath == "" {
		return moduleInfo{}, false, fmt.Errorf("Go package URL has an empty module path")
	}

	modulePath, err = url.PathUnescape(modulePath)
	if err != nil {
		return moduleInfo{}, false, fmt.Errorf("unescaping Go module path: %w", err)
	}
	if moduleVersion != "(devel)" {
		moduleVersion, err = url.PathUnescape(moduleVersion)
		if err != nil {
			return moduleInfo{}, false, fmt.Errorf("unescaping Go module version: %w", err)
		}
		if isUnprefixedSemver(moduleVersion) {
			moduleVersion = "v" + moduleVersion
		}
	}

	var moduleSum string
	if u.RawQuery != "" {
		values, err := url.ParseQuery(u.RawQuery)
		if err != nil {
			return moduleInfo{}, false, fmt.Errorf("parsing Go package URL qualifiers: %w", err)
		}
		moduleSum = values.Get("checksum")
	}

	return moduleInfo{path: modulePath, version: moduleVersion, sum: moduleSum}, true, nil
}

type stampEntry struct {
	key   string
	value string
}

func vcsStamp(args []string) error {
	args, _, err := expandParamsFiles(args)
	if err != nil {
		return err
	}

	flags := flag.NewFlagSet("vcsstamp", flag.ExitOnError)
	input := flags.String("in", "", "The stable workspace status file to filter.")
	output := flags.String("out", "", "The filtered VCS stamp file to write.")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if *input == "" {
		return fmt.Errorf("-in must be set")
	}
	if *output == "" {
		return fmt.Errorf("-out must be set")
	}

	stampMap, err := readStampMap([]string{*input})
	if err != nil {
		return err
	}

	entries := buildInfoVCSEntries(stampMap)
	var builder strings.Builder
	// Bazel's workspace status format is one key/value pair per line and
	// explicitly forbids either part from spanning lines. Values therefore
	// cannot contain newlines here, while the first-space separator preserves
	// all other permitted characters in the value.
	for _, entry := range entries {
		builder.WriteString(entry.key)
		builder.WriteByte(' ')
		builder.WriteString(entry.value)
		builder.WriteByte('\n')
	}
	return os.WriteFile(*output, []byte(builder.String()), 0o666)
}

func readStampMap(files []string) (map[string]string, error) {
	stampMap := map[string]string{}
	for _, stampFile := range files {
		stampBuf, err := os.ReadFile(stampFile)
		if err != nil {
			return nil, fmt.Errorf("failed reading stamp file %s: %v", stampFile, err)
		}
		if len(stampBuf) == 0 {
			continue
		}
		lines := bytes.Split(stampBuf, []byte{'\n'})
		if len(lines[len(lines)-1]) == 0 {
			lines = lines[:len(lines)-1]
		}
		for _, stampLine := range lines {
			stampLine = bytes.TrimSuffix(stampLine, []byte{'\r'})
			line := strings.SplitN(string(stampLine), " ", 2)
			switch len(line) {
			case 0:
				// Nothing to do here.
			case 1:
				stampMap[line[0]] = ""
			case 2:
				stampMap[line[0]] = line[1]
			}
		}
	}
	return stampMap, nil
}

func buildInfoDeps(modules []moduleInfo, mainModule moduleInfo) []*debug.Module {
	// Package metadata is not guaranteed to come from a single MVS-resolved
	// module graph. Preserve distinct versions for the same path rather than
	// silently choosing one when independently authored metadata conflicts.
	seen := map[moduleInfo]struct{}{}
	unique := make([]moduleInfo, 0, len(modules))
	for _, module := range modules {
		if module.path == "" || module.version == "" {
			continue
		}
		if mainModule.path != "" && module.path == mainModule.path {
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
		if unique[i].version != unique[j].version {
			return unique[i].version < unique[j].version
		}
		return unique[i].sum < unique[j].sum
	})

	deps := make([]*debug.Module, 0, len(unique))
	for _, module := range unique {
		deps = append(deps, &debug.Module{
			Path:    module.path,
			Version: module.version,
			Sum:     module.sum,
		})
	}
	return deps
}

func buildInfoMain(module moduleInfo) debug.Module {
	if module.path == "" {
		return debug.Module{}
	}
	return debug.Module{Path: module.path, Version: module.version}
}

func buildInfoSettings(buildmode string, buildTags []string, race, msan, cover, go123ArchSettings bool, mainModulePath string, stampMap map[string]string) []debug.BuildSetting {
	return buildInfoSettingsFromEnv(buildmode, buildTags, race, msan, cover, go123ArchSettings, mainModulePath, os.Getenv, stampMap)
}

func buildInfoSettingsFromEnv(buildmode string, buildTags []string, race, msan, cover, go123ArchSettings bool, mainModulePath string, getenv func(string) string, stampMap map[string]string) []debug.BuildSetting {
	if buildmode == "" || buildmode == "normal" {
		buildmode = "exe"
	}

	settings := []debug.BuildSetting{
		{Key: "-buildmode", Value: buildmode},
		{Key: "-compiler", Value: "gc"},
	}
	if cover {
		settings = append(settings, debug.BuildSetting{Key: "-cover", Value: "true"})
	}
	if msan {
		settings = append(settings, debug.BuildSetting{Key: "-msan", Value: "true"})
	}
	if race {
		settings = append(settings, debug.BuildSetting{Key: "-race", Value: "true"})
	}
	if buildTags = buildInfoTags(buildTags, race, msan); len(buildTags) > 0 {
		settings = append(settings, debug.BuildSetting{Key: "-tags", Value: strings.Join(buildTags, ",")})
	}
	settings = append(settings, debug.BuildSetting{Key: "-trimpath", Value: "true"})

	goarch := ""
	for _, key := range []string{"CGO_ENABLED", "GOARCH", "GOEXPERIMENT", "GOOS"} {
		if value := getenv(key); value != "" {
			if key == "GOARCH" {
				goarch = value
			}
			settings = append(settings, debug.BuildSetting{Key: key, Value: value})
		}
	}
	if key, value := buildInfoArchSetting(goarch, go123ArchSettings, getenv); key != "" && value != "" {
		settings = append(settings, debug.BuildSetting{Key: key, Value: value})
	}
	settings = append(settings, buildInfoVCSSettings(mainModulePath, stampMap)...)
	return settings
}

func buildInfoVCSSettings(mainModulePath string, stampMap map[string]string) []debug.BuildSetting {
	if mainModulePath == "" {
		return nil
	}
	vcsEntries := buildInfoVCSEntries(stampMap)
	if len(vcsEntries) == 0 {
		return nil
	}

	settings := []debug.BuildSetting{{Key: "vcs", Value: vcsEntries[0].value}}
	for _, entry := range vcsEntries[1:] {
		switch entry.key {
		case stableVCSRevisionKey:
			settings = append(settings, debug.BuildSetting{Key: "vcs.revision", Value: entry.value})
		case stableVCSTimeKey:
			settings = append(settings, debug.BuildSetting{Key: "vcs.time", Value: entry.value})
		case stableVCSModifiedKey:
			settings = append(settings, debug.BuildSetting{Key: "vcs.modified", Value: entry.value})
		}
	}
	return settings
}

func buildInfoVCSEntries(stampMap map[string]string) []stampEntry {
	vcs := strings.TrimSpace(stampMap[stableVCSKey])
	if vcs == "" {
		return nil
	}

	entries := []stampEntry{{key: stableVCSKey, value: vcs}}
	if revision := strings.TrimSpace(stampMap[stableVCSRevisionKey]); revision != "" {
		entries = append(entries, stampEntry{key: stableVCSRevisionKey, value: revision})
	}
	if stamp := strings.TrimSpace(stampMap[stableVCSTimeKey]); stamp != "" {
		if parsed, err := time.Parse(time.RFC3339Nano, stamp); err == nil {
			entries = append(entries, stampEntry{key: stableVCSTimeKey, value: parsed.UTC().Format(time.RFC3339Nano)})
		}
	}
	switch modified := strings.TrimSpace(stampMap[stableVCSModifiedKey]); modified {
	case "false", "true":
		entries = append(entries, stampEntry{key: stableVCSModifiedKey, value: modified})
	}
	return entries
}

func buildInfoTags(buildTags []string, race, msan bool) []string {
	if len(buildTags) == 0 {
		return nil
	}
	filtered := append([]string(nil), buildTags...)
	if race {
		filtered = stripLastBuildTag(filtered, "race")
	}
	if msan {
		filtered = stripLastBuildTag(filtered, "msan")
	}
	return filtered
}

func stripLastBuildTag(buildTags []string, want string) []string {
	for i := len(buildTags) - 1; i >= 0; i-- {
		if buildTags[i] == want {
			return append(buildTags[:i], buildTags[i+1:]...)
		}
	}
	return buildTags
}

func buildInfoArchSetting(goarch string, go123ArchSettings bool, getenv func(string) string) (string, string) {
	switch goarch {
	case "386":
		return "GO386", firstNonEmpty(getenv("GO386"), "sse2")
	case "amd64":
		return "GOAMD64", firstNonEmpty(getenv("GOAMD64"), "v1")
	case "arm":
		if value := getenv("GOARM"); value != "" {
			return "GOARM", value
		}
		if getenv("GOOS") == "android" {
			return "GOARM", "7"
		}
		// The Go SDK's default GOARM may be detected when the SDK is built.
		// Omit it when unknown instead of deriving it from the execution platform.
	case "arm64":
		if go123ArchSettings {
			return "GOARM64", firstNonEmpty(getenv("GOARM64"), "v8.0")
		}
	case "mips", "mipsle":
		return "GOMIPS", firstNonEmpty(getenv("GOMIPS"), "hardfloat")
	case "mips64", "mips64le":
		return "GOMIPS64", firstNonEmpty(getenv("GOMIPS64"), "hardfloat")
	case "ppc64", "ppc64le":
		return "GOPPC64", firstNonEmpty(getenv("GOPPC64"), "power8")
	case "riscv64":
		if go123ArchSettings {
			return "GORISCV64", firstNonEmpty(getenv("GORISCV64"), "rva20u64")
		}
	case "wasm":
		if value := getenv("GOWASM"); value != "" {
			return "GOWASM", value
		}
	}
	return "", ""
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}

func modInfoData(path string, mainModule moduleInfo, settings []debug.BuildSetting, modules []moduleInfo) string {
	info := &debug.BuildInfo{
		Path:     path,
		Main:     buildInfoMain(mainModule),
		Deps:     buildInfoDeps(modules, mainModule),
		Settings: settings,
	}
	return buildInfoStart + info.String() + buildInfoEnd
}

func shouldEmitBuildInfo(buildmode string) bool {
	switch buildmode {
	case "c-archive", "c-shared", "plugin":
		return false
	default:
		return true
	}
}
