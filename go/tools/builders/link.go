// Copyright 2017 The Bazel Authors. All rights reserved.
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

// link combines the results of a compile step using "go tool link". It is invoked by the
// Go rules as an action.
package main

import (
	"bufio"
	"bytes"
	"flag"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
)

func link(args []string) error {
	// Parse arguments.
	args, _, err := expandParamsFiles(args)
	if err != nil {
		return err
	}
	builderArgs, toolArgs := splitArgs(args)
	stamps := multiFlag{}
	xdefs := multiFlag{}
	archives := archiveMultiFlag{}
	flags := flag.NewFlagSet("link", flag.ExitOnError)
	goenv := envFlags(flags)
	main := flags.String("main", "", "Path to the main archive.")
	packagePath := flags.String("p", "", "Package path of the main archive.")
	outFile := flags.String("o", "", "Path to output file.")
	flags.Var(&archives, "arc", "Label, package path, and file name of a dependency, separated by '='")
	packageList := flags.String("package_list", "", "The file containing the list of standard library packages")
	buildmode := flags.String("buildmode", "", "Build mode used.")
	flags.Var(&xdefs, "X", "A string variable to replace in the linked binary (repeated).")
	flags.Var(&stamps, "stamp", "The name of a file with stamping values.")
	conflictErrMsg := flags.String("conflict_err", "", "Error message about conflicts to report if there's a link error.")
	buildinfoFile := flags.String("buildinfo", "", "Path to buildinfo dependency file for Go 1.18+ buildInfo.")
	versionMapFile := flags.String("versionmap", "", "Path to version map file with real dependency versions from package_info.")
	bazelTarget := flags.String("bazeltarget", "", "Bazel target label for buildInfo metadata.")
	if err := flags.Parse(builderArgs); err != nil {
		return err
	}
	if err := goenv.checkFlagsAndSetGoroot(); err != nil {
		return err
	}

	// On Windows, take the absolute path of the output file and main file.
	// This is needed on Windows because the relative path is frequently too long.
	// os.Open on Windows converts absolute paths to some other path format with
	// longer length limits. Absolute paths do not work on macOS for .dylib
	// outputs because they get baked in as the "install path".
	if runtime.GOOS != "darwin" && runtime.GOOS != "ios" {
		*outFile = abs(*outFile)
	}
	*main = abs(*main)

	// If we were given any stamp value files, read and parse them
	stampMap := map[string]string{}
	for _, stampfile := range stamps {
		stampbuf, err := ioutil.ReadFile(stampfile)
		if err != nil {
			return fmt.Errorf("Failed reading stamp file %s: %v", stampfile, err)
		}
		scanner := bufio.NewScanner(bytes.NewReader(stampbuf))
		for scanner.Scan() {
			line := strings.SplitN(scanner.Text(), " ", 2)
			switch len(line) {
			case 0:
				// Nothing to do here
			case 1:
				// Map to the empty string
				stampMap[line[0]] = ""
			case 2:
				// Key and value
				stampMap[line[0]] = line[1]
			}
		}
	}

	// Parse version map file if provided (real versions from package_info)
	versionMap := make(map[string]string)
	if *versionMapFile != "" {
		versionMapData, err := ioutil.ReadFile(*versionMapFile)
		if err != nil {
			return fmt.Errorf("Failed reading version map file %s: %v", *versionMapFile, err)
		}

		// Parse the version map file: importpath\tversion
		lines := strings.Split(string(versionMapData), "\n")
		for _, line := range lines {
			line = strings.TrimSpace(line)
			if line == "" {
				continue
			}
			parts := strings.Split(line, "\t")
			if len(parts) >= 2 {
				importpath := parts[0]
				version := parts[1]
				versionMap[importpath] = version
			}
		}
	}

	// Parse buildinfo file if provided (for Go 1.18+ dependency metadata)
	// Merge with version map to replace v0.0.0 with real versions
	var deps []*Module
	if *buildinfoFile != "" {
		buildinfoData, err := ioutil.ReadFile(*buildinfoFile)
		if err != nil {
			return fmt.Errorf("Failed reading buildinfo file %s: %v", *buildinfoFile, err)
		}

		// Parse the buildinfo file to extract dependency information
		// Format: tab-separated lines with "path", "dep", etc.
		// First pass: extract the main module path
		mainModulePath := ""
		lines := strings.Split(string(buildinfoData), "\n")
		for _, line := range lines {
			line = strings.TrimSpace(line)
			if line == "" {
				continue
			}
			parts := strings.Split(line, "\t")
			if len(parts) >= 2 && parts[0] == "path" {
				mainModulePath = parts[1]
				break
			}
		}

		// Second pass: collect dependencies
		for _, line := range lines {
			line = strings.TrimSpace(line)
			if line == "" {
				continue
			}
			parts := strings.Split(line, "\t")
			if len(parts) >= 3 && parts[0] == "dep" {
				importpath := parts[1]
				version := parts[2]

				// Skip internal packages (from main module)
				// Filter out packages that are part of the main module being built
				if mainModulePath != "" && strings.HasPrefix(importpath, mainModulePath+"/") {
					continue
				}

				// Replace (devel) sentinel with real version from version map
				if version == "(devel)" {
					// First try exact match
					if realVersion, ok := versionMap[importpath]; ok && realVersion != "" {
						version = realVersion
					} else {
						// Try to find parent module version for subpackages
						// Check progressively shorter prefixes to find the module root
						bestMatch := ""
						for modulePath := range versionMap {
							if strings.HasPrefix(importpath, modulePath+"/") {
								// Found a parent module, keep the longest match
								if len(modulePath) > len(bestMatch) {
									bestMatch = modulePath
								}
							}
						}
						if bestMatch != "" {
							version = versionMap[bestMatch]
						}
					}
				}

				// Skip dependencies that still have (devel) after version resolution
				// These are internal packages from the monorepo without real versions
				// (devel) is an invalid semantic version used as a sentinel
				if version == "(devel)" {
					continue
				}

				// Format: dep\t<importpath>\t<version>
				deps = append(deps, &Module{
					Path:    importpath,
					Version: version,
					Sum:     "", // No checksum in Bazel builds
				})
			}
		}
	}

	// Prepare link config for buildinfo generation
	var realBuildMode string
	if *buildmode == "" {
		realBuildMode = "exe"
	} else {
		realBuildMode = *buildmode
	}

	cgoEnabled := os.Getenv("CGO_ENABLED") == "1"
	goarch := os.Getenv("GOARCH")
	goos := os.Getenv("GOOS")

	// Extract architecture feature level
	goarchFeatureKey := ""
	goarchFeatureValue := ""
	switch goarch {
	case "amd64":
		if v := os.Getenv("GOAMD64"); v != "" {
			goarchFeatureKey = "GOAMD64"
			goarchFeatureValue = v
		}
	case "arm":
		if v := os.Getenv("GOARM"); v != "" {
			goarchFeatureKey = "GOARM"
			goarchFeatureValue = v
		}
	case "386":
		if v := os.Getenv("GO386"); v != "" {
			goarchFeatureKey = "GO386"
			goarchFeatureValue = v
		}
	case "mips", "mipsle":
		if v := os.Getenv("GOMIPS"); v != "" {
			goarchFeatureKey = "GOMIPS"
			goarchFeatureValue = v
		}
	case "mips64", "mips64le":
		if v := os.Getenv("GOMIPS64"); v != "" {
			goarchFeatureKey = "GOMIPS64"
			goarchFeatureValue = v
		}
	case "ppc64", "ppc64le":
		if v := os.Getenv("GOPPC64"); v != "" {
			goarchFeatureKey = "GOPPC64"
			goarchFeatureValue = v
		}
	case "riscv64":
		if v := os.Getenv("GORISCV64"); v != "" {
			goarchFeatureKey = "GORISCV64"
			goarchFeatureValue = v
		}
	case "wasm":
		if v := os.Getenv("GOWASM"); v != "" {
			goarchFeatureKey = "GOWASM"
			goarchFeatureValue = v
		}
	}

	// Extract CGO flags
	cgoCflags := ""
	cgoCxxflags := ""
	cgoLdflags := ""
	if cgoEnabled {
		cgoCflags = os.Getenv("CGO_CFLAGS")
		cgoCxxflags = os.Getenv("CGO_CXXFLAGS")
		cgoLdflags = os.Getenv("CGO_LDFLAGS")
	}

	cfg := linkConfig{
		path:               *packagePath,
		buildMode:          realBuildMode,
		compiler:           "gc",
		cgoEnabled:         cgoEnabled,
		goos:               goos,
		goarch:             goarch,
		pgoProfilePath:     "",     // Will be set below if pgoprofile is provided
		buildinfoFile:      *buildinfoFile,
		deps:               deps,
		goarchFeatureKey:   goarchFeatureKey,
		goarchFeatureValue: goarchFeatureValue,
		cgoCflags:          cgoCflags,
		cgoCxxflags:        cgoCxxflags,
		cgoLdflags:         cgoLdflags,
		bazelTarget:        *bazelTarget,
	}

	// Build an importcfg file.
	importcfgName, err := buildImportcfgFileForLink(archives, *packageList, goenv.installSuffix, filepath.Dir(*outFile), cfg)
	if err != nil {
		return err
	}
	if !goenv.shouldPreserveWorkDir {
		defer os.Remove(importcfgName)
	}

	// generate any additional link options we need
	goargs := goenv.goTool("link")
	goargs = append(goargs, "-importcfg", importcfgName)

	parseXdef := func(xdef string) (pkg, name, value string, err error) {
		eq := strings.IndexByte(xdef, '=')
		if eq < 0 {
			return "", "", "", fmt.Errorf("-X flag does not contain '=': %s", xdef)
		}
		dot := strings.LastIndexByte(xdef[:eq], '.')
		if dot < 0 {
			return "", "", "", fmt.Errorf("-X flag does not contain '.': %s", xdef)
		}
		pkg, name, value = xdef[:dot], xdef[dot+1:eq], xdef[eq+1:]
		if pkg == *packagePath {
			pkg = "main"
		}
		return pkg, name, value, nil
	}
	for _, xdef := range xdefs {
		pkg, name, value, err := parseXdef(xdef)
		if err != nil {
			return err
		}
		var missingKey bool
		value = regexp.MustCompile(`\{.+?\}`).ReplaceAllStringFunc(value, func(key string) string {
			if value, ok := stampMap[key[1:len(key)-1]]; ok {
				return value
			}
			missingKey = true
			return key
		})
		if !missingKey {
			goargs = append(goargs, "-X", fmt.Sprintf("%s.%s=%s", pkg, name, value))
		}
	}

	if *buildmode != "" {
		goargs = append(goargs, "-buildmode", *buildmode)
	}
	goargs = append(goargs, "-o", *outFile)

	// substitute `builder cc` for the linker with a symlink to builder called `builder-cc`.
	// unfortunately we can't just set an environment variable to `builder cc` because
	// in `go tool link` the `linkerFlagSupported` [1][2] call sites used to determine
	// if a linker supports various flags all appear to use the first arg after splitting
	// so the `cc` would be left off of `builder cc`
	//
	//    [1]: https://cs.opensource.google/go/go/+/ad7f736d8f51ea03166b698256385c869968ae3e:src/cmd/link/internal/ld/lib.go;l=1739
	//    [2]: https://cs.opensource.google/go/go/+/master:src/cmd/link/internal/ld/lib.go;drc=c6531fae589cf3f9475f3567a5beffb4336fe1d6;l=1429?q=linkerFlagSupported&ss=go%2Fgo
	linkerCleanup, err := absCCLinker(toolArgs)
	if err != nil {
		return err
	}
	defer linkerCleanup()
	// add in the unprocess pass through options
	goargs = append(goargs, toolArgs...)
	goargs = append(goargs, *main)

	clearGoRoot, err := onVersion(23)
	if err != nil {
		return err
	}
	if clearGoRoot {
		// Explicitly set GOROOT to a dummy value when running linker.
		// This ensures that the GOROOT written into the binary
		// is constant and thus builds are reproducible.
		oldroot := os.Getenv("GOROOT")
		os.Setenv("GOROOT", "GOROOT")
		defer os.Setenv("GOROOT", oldroot)
	}
	if err := goenv.runCommand(goargs); err != nil {
		return err
	}

	if *buildmode == "c-archive" {
		if err := stripArMetadata(*outFile); err != nil {
			return fmt.Errorf("error stripping archive metadata: %v", err)
		}
	}

	return nil
}

var versionExp = regexp.MustCompile(`.*go1\.(\d+).*$`)

func onVersion(version int) (bool, error) {
	v := runtime.Version()
	m := versionExp.FindStringSubmatch(v)
	if len(m) != 2 {
		return false, fmt.Errorf("failed to match against Go version %q", v)
	}
	mvStr := m[1]
	mv, err := strconv.Atoi(mvStr)
	if err != nil {
		return false, fmt.Errorf("convert minor version %q to int: %w", mvStr, err)
	}

	return mv >= version, nil
}
