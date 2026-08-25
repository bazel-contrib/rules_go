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
	"crypto/md5"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

// baselineCoverVar is the prefix cmd/cover uses for the counter variables in
// the throwaway instrumented sources this action produces. They are never
// compiled; only the coverage meta-data file is read back.
const baselineCoverVar = "GoBaselineCover"

// baselineCoverMode is the mode cmd/cover is invoked with. Any mode yields the
// same block positions -- the mode only decides how counts are recorded at run
// time, and no baseline block is ever executed -- so the cheapest is used.
const baselineCoverMode = "set"

// baselineCoverage writes a "baseline" LCOV tracefile for a Go package: every
// coverable line in the package, each with an execution count of zero.
//
// Bazel requests a baseline so that a package no test ever links still appears
// in the combined coverage report. Absent one, Bazel substitutes a stub that
// names the source file and nothing else, which its LcovPrinter renders as
// "LF:0/LH:0". A consumer cannot tell that apart from a file with genuinely
// nothing to cover, so wholly untested packages report 100% instead of 0%.
// See https://github.com/bazelbuild/bazel/issues/5716.
//
// The line set comes from "go tool cover" rather than from an independent
// source analysis, so it agrees exactly with what a real coverage run reports —
// including multi-line statements, case clauses and closing braces, which a
// statement-position approximation gets wrong.
func baselineCoverage(args []string) error {
	args, _, err := expandParamsFiles(args)
	if err != nil {
		return err
	}

	fs := flag.NewFlagSet("GoBaselineCoverage", flag.ExitOnError)
	goenv := envFlags(fs)
	var unfilteredSrcs multiFlag
	var outPath, covdataPath, importPath string
	fs.Var(&unfilteredSrcs, "src", "A source file to consider for coverage. May be repeated.")
	fs.StringVar(&outPath, "o", "", "The LCOV tracefile to write.")
	fs.StringVar(&covdataPath, "covdata", "", "The path to the covdata tool.")
	fs.StringVar(&importPath, "importpath", "", "The import path of the package. May be empty.")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if err := goenv.checkFlagsAndSetGoroot(); err != nil {
		return err
	}
	if outPath == "" {
		return fmt.Errorf("-o is required")
	}
	if covdataPath == "" {
		return fmt.Errorf("-covdata is required")
	}

	// Apply the same build-constraint filtering the compile action applies, so
	// a file excluded on this GOOS/GOARCH is never reported as uncovered.
	srcs, err := filterAndSplitFiles(unfilteredSrcs)
	if err != nil {
		return err
	}

	lcov, err := baselineLCOV(goenv, covdataPath, importPath, srcs.goSrcs)
	if err != nil {
		return err
	}
	return os.WriteFile(outPath, []byte(lcov), writeFileMode)
}

// baselineLCOV renders the coverable lines of goSrcs as LCOV with every count
// zero. Sources excluded by build constraints have already been filtered out
// and so are absent, correctly: they are not compiled on this platform and
// nothing about them is coverable here.
func baselineLCOV(goenv *env, covdataPath, importPath string, goSrcs []fileInfo) (string, error) {
	if len(goSrcs) == 0 {
		return "", nil
	}

	workDir, cleanup, err := goenv.workDir()
	if err != nil {
		return "", err
	}
	defer cleanup()

	linesByFile, err := coverableLines(goenv, covdataPath, importPath, workDir, goSrcs)
	if err != nil {
		return "", err
	}

	var out strings.Builder
	for _, src := range goSrcs {
		// Bazel merges LCOV across languages and requires exec-root-relative
		// source paths. The filenames handed to this action already are, and
		// compilepkg's lcov cover_format uses the very same strings.
		//
		// A file with nothing to cover is still named, with LF:0. That matches
		// the shape of the stub this replaces, but now the zero is a measured
		// fact rather than the absence of a measurement.
		name := filepath.ToSlash(src.filename)
		lines := linesByFile[name]
		delete(linesByFile, name)
		fmt.Fprintf(&out, "SF:%s\n", name)
		for _, line := range lines {
			fmt.Fprintf(&out, "DA:%d,0\n", line)
		}
		fmt.Fprintf(&out, "LH:0\nLF:%d\nend_of_record\n", len(lines))
	}
	for name := range linesByFile {
		return "", fmt.Errorf("cover reported coverable lines in %s, which is not among the package's sources", name)
	}
	return out.String(), nil
}

// coverableLines instruments the package once with cmd/cover's -pkgcfg mode,
// which emits a coverage meta-data file, and decodes that file with the
// covdata tool into the set of coverable lines per source file, sorted and
// deduplicated.
//
// cgo sources are included. compilepkg instruments them before cgo rewrites
// them, so a measured run reports positions in the unprocessed source -- the
// same ones read back here. Cgo sources that are not compiled at all, because
// CGO_ENABLED is 0, have already been filtered out.
func coverableLines(goenv *env, covdataPath, importPath, workDir string, goSrcs []fileInfo) (map[string][]int, error) {
	// cmd/cover refuses an empty PkgPath, but with Local set the file names it
	// records never include it, so a placeholder serves for the packages that
	// have no import path, such as most binaries.
	if importPath == "" {
		importPath = "baseline-coverage"
	}

	metaDir := filepath.Join(workDir, "covmeta")
	if err := os.Mkdir(metaDir, 0o777); err != nil {
		return nil, err
	}

	// The file name must match the covmeta.<tag> shape covdata looks for. The
	// tag itself only distinguishes multiple meta files in one directory, of
	// which there is one.
	metaFile := filepath.Join(metaDir, fmt.Sprintf("covmeta.%x", md5.Sum([]byte(importPath))))

	// EmitMetaFile exists for "go test -cover" on a package with no test
	// files, which needs the block positions of a package that is never
	// linked into a test binary -- exactly this action's situation. Local
	// makes cmd/cover record each source file under the name it was given on
	// the command line, which is already exec-root-relative, rather than under
	// <importpath>/<basename>.
	cfg := coverPkgConfig{
		OutConfig:    filepath.Join(workDir, "outcfg.txt"),
		PkgPath:      importPath,
		PkgName:      goSrcs[0].pkg,
		Granularity:  "perblock",
		Local:        true,
		EmitMetaFile: metaFile,
	}
	cfgData, err := json.Marshal(cfg)
	if err != nil {
		return nil, err
	}
	pkgcfgPath := filepath.Join(workDir, "pkgcfg.json")
	if err := os.WriteFile(pkgcfgPath, cfgData, writeFileMode); err != nil {
		return nil, err
	}

	// cmd/cover insists on writing instrumented output: a covervars.go
	// followed by one file per input. All of it is thrown away with the work
	// directory; only the meta-data file is of interest.
	var outList strings.Builder
	fmt.Fprintf(&outList, "%s\n", filepath.Join(workDir, "covervars.go"))
	for i := range goSrcs {
		fmt.Fprintf(&outList, "%s\n", filepath.Join(workDir, fmt.Sprintf("baseline.%d.go", i)))
	}
	outListPath := filepath.Join(workDir, "outfilelist.txt")
	if err := os.WriteFile(outListPath, []byte(outList.String()), writeFileMode); err != nil {
		return nil, err
	}

	goargs := goenv.goTool("cover", "-pkgcfg", pkgcfgPath, "-var", baselineCoverVar, "-mode", baselineCoverMode, "-outfilelist", outListPath)
	for _, src := range goSrcs {
		goargs = append(goargs, src.filename)
	}
	if err := goenv.runCommand(goargs); err != nil {
		return nil, fmt.Errorf("instrumenting package: %w", err)
	}

	// cmd/cover always creates the meta-data file, but leaves it empty for a
	// package with no function bodies, which genuinely has no coverable lines.
	// covdata cannot decode an empty file, and has nothing to say about one.
	// An absent file means cmd/cover no longer honors EmitMetaFile, and that
	// failure must be loud: quietly reporting LF:0 everywhere is the very
	// state this action exists to fix.
	info, err := os.Stat(metaFile)
	if err != nil {
		return nil, fmt.Errorf("the cover tool did not emit a coverage meta-data file for %s: %w", importPath, err)
	}
	if info.Size() == 0 {
		return nil, nil
	}

	profilePath := filepath.Join(workDir, "baseline.cov")
	covdataArgs := []string{covdataPath, "textfmt", "-i=" + metaDir, "-o=" + profilePath}
	if err := goenv.runCommand(covdataArgs); err != nil {
		return nil, fmt.Errorf("decoding coverage meta-data for %s: %w", importPath, err)
	}
	profile, err := os.ReadFile(profilePath)
	if err != nil {
		return nil, err
	}
	return parseCoverProfile(string(profile))
}

// coverLinePattern matches one block line of a coverage profile. It is copied
// verbatim from bzltestutil's converter, which parses the same text format for
// measured coverage, so that both readers accept exactly the same input. The
// pattern cannot be shared: builder is a go_tool_binary restricted to the
// standard library, and bzltestutil imports coverdata.
//
// The greedy path group is what allows a source path to contain a space, which
// splitting on whitespace would break.
var coverLinePattern = regexp.MustCompile(`^(?P<path>.+):(?P<startLine>\d+)\.(?P<startColumn>\d+),(?P<endLine>\d+)\.(?P<endColumn>\d+) (?P<numStmt>\d+) (?P<count>\d+)$`)

// parseCoverProfile extracts the per-file line sets from a coverage profile in
// the text format emitted by "covdata textfmt": a "mode:" header followed by
// one block per line,
//
//	name.go:startLine.startCol,endLine.endCol numStmt count
//
// Every line a block spans counts as coverable, matching how a measured
// profile is rendered as LCOV after a coverage run.
func parseCoverProfile(profile string) (map[string][]int, error) {
	seen := make(map[string]map[int]bool)
	// Read by lines the way bzltestutil does, so that a trailing carriage
	// return is stripped rather than left to defeat the pattern's anchor.
	scanner := bufio.NewScanner(strings.NewReader(profile))
	for scanner.Scan() {
		line := scanner.Text()
		m := coverLinePattern.FindStringSubmatch(line)
		if m == nil {
			// Matching before this check, as bzltestutil does, so that a
			// source path beginning with "mode:" is read as the record it is
			// rather than mistaken for the header.
			if line == "" || strings.HasPrefix(line, "mode: ") {
				continue
			}
			return nil, fmt.Errorf("malformed line in coverage profile: %q", line)
		}
		// The paths are compared against the source names this action was
		// given, which are slash-separated.
		name := filepath.ToSlash(m[1])
		startLine, err := strconv.Atoi(m[2])
		if err != nil {
			return nil, fmt.Errorf("malformed start line in coverage profile line %q: %v", line, err)
		}
		endLine, err := strconv.Atoi(m[4])
		if err != nil {
			return nil, fmt.Errorf("malformed end line in coverage profile line %q: %v", line, err)
		}
		if startLine <= 0 || endLine < startLine {
			return nil, fmt.Errorf("nonsensical block span %d-%d in coverage profile line %q", startLine, endLine, line)
		}
		if seen[name] == nil {
			seen[name] = make(map[int]bool)
		}
		for l := startLine; l <= endLine; l++ {
			seen[name][l] = true
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("reading coverage profile: %w", err)
	}

	linesByFile := make(map[string][]int, len(seen))
	for name, set := range seen {
		lines := make([]int, 0, len(set))
		for l := range set {
			lines = append(lines, l)
		}
		sort.Ints(lines)
		linesByFile[name] = lines
	}
	return linesByFile, nil
}
