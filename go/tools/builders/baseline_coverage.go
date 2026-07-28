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
	"flag"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

// baselineCoverVar is the variable name given to the coverage struct in the
// throwaway instrumented sources this action produces. It is never compiled;
// only the position table inside it is read back.
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
	var outPath string
	fs.Var(&unfilteredSrcs, "src", "A source file to consider for coverage. May be repeated.")
	fs.StringVar(&outPath, "o", "", "The LCOV tracefile to write.")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if err := goenv.checkFlagsAndSetGoroot(); err != nil {
		return err
	}
	if outPath == "" {
		return fmt.Errorf("-o is required")
	}

	// Apply the same build-constraint filtering the compile action applies, so
	// a file excluded on this GOOS/GOARCH is never reported as uncovered.
	srcs, err := filterAndSplitFiles(unfilteredSrcs)
	if err != nil {
		return err
	}

	lcov, err := baselineLCOV(goenv, srcs.goSrcs)
	if err != nil {
		return err
	}
	return os.WriteFile(outPath, []byte(lcov), writeFileMode)
}

// baselineLCOV renders the coverable lines of goSrcs as LCOV with every count
// zero. Sources excluded by build constraints have already been filtered out
// and so are absent, correctly: they are not compiled on this platform and
// nothing about them is coverable here.
func baselineLCOV(goenv *env, goSrcs []fileInfo) (string, error) {
	if len(goSrcs) == 0 {
		return "", nil
	}

	workDir, cleanup, err := goenv.workDir()
	if err != nil {
		return "", err
	}
	defer cleanup()

	var out strings.Builder
	for i, src := range goSrcs {
		// cgo sources are included. compilepkg instruments them before cgo
		// rewrites them, so a measured run reports positions in the
		// unprocessed source -- the same ones read back here. Cgo sources
		// that are not compiled at all, because CGO_ENABLED is 0, have
		// already been filtered out.
		lines, err := coverableLines(goenv, workDir, i, src.filename)
		if err != nil {
			return "", err
		}

		// Bazel merges LCOV across languages and requires exec-root-relative
		// source paths. The filenames handed to this action already are, and
		// compilepkg's lcov cover_format uses the very same strings.
		//
		// A file with nothing to cover is still named, with LF:0. That matches
		// the shape of the stub this replaces, but now the zero is a measured
		// fact rather than the absence of a measurement.
		fmt.Fprintf(&out, "SF:%s\n", filepath.ToSlash(src.filename))
		for _, line := range lines {
			fmt.Fprintf(&out, "DA:%d,0\n", line)
		}
		fmt.Fprintf(&out, "LH:0\nLF:%d\nend_of_record\n", len(lines))
	}
	return out.String(), nil
}

// coverableLines instruments one source file and reads back the set of lines
// the instrumentation covers, sorted and deduplicated.
func coverableLines(goenv *env, workDir string, index int, srcName string) ([]int, error) {
	instrumented := filepath.Join(workDir, fmt.Sprintf("baseline.%d.%s", index, filepath.Base(srcName)))

	// The single-file form of cmd/cover is used deliberately over the -pkgcfg
	// form. -pkgcfg can emit a coverage meta-data file directly (its
	// EmitMetaFile field exists for "go test -cover" on a package with no test
	// files, exactly this situation), but decoding that file requires the
	// covdata tool, which the Go SDK does not ship prebuilt -- the go command
	// compiles it on demand into the user's build cache. An action cannot rely
	// on that. The single-file form needs only pkg/tool/<platform>/cover, which
	// rules_go already depends on, and its position table carries the same
	// blocks -pkgcfg would have recorded.
	goargs := goenv.goTool("cover", "-mode", baselineCoverMode, "-var", baselineCoverVar, "-o", instrumented, srcName)
	if err := goenv.runCommand(goargs); err != nil {
		return nil, fmt.Errorf("instrumenting %s: %w", srcName, err)
	}

	blocks, err := parseCoverPositions(instrumented)
	if err != nil {
		return nil, fmt.Errorf("reading coverage positions for %s: %w", srcName, err)
	}

	seen := make(map[int]struct{})
	for _, b := range blocks {
		for line := b.startLine; line <= b.endLine; line++ {
			seen[line] = struct{}{}
		}
	}
	lines := make([]int, 0, len(seen))
	for line := range seen {
		lines = append(lines, line)
	}
	sort.Ints(lines)
	return lines, nil
}

// coverBlock is one instrumented basic block's line span.
type coverBlock struct {
	startLine int
	endLine   int
}

// parseCoverPositions extracts the block spans from a file instrumented by
// cmd/cover. The instrumented source ends with a declaration of the form
//
//	var GoBaselineCover = struct {
//		Count   [8]uint32
//		Pos     [3 * 8]uint32
//		NumStmt [8]uint16
//	}{
//		Pos: [3 * 8]uint32{
//			22, 24, 0x16001d, // [0]
//			...
//		},
//		...
//	}
//
// where Pos holds one triple per block: start line, end line, and the two
// columns packed into a single word. Only the line numbers are of interest.
//
// A file with no functions is instrumented into a file with no such
// declaration; that yields no blocks and is not an error.
func parseCoverPositions(path string) ([]coverBlock, error) {
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, path, nil, parser.SkipObjectResolution)
	if err != nil {
		return nil, err
	}

	lit := findCoverPosLiteral(f)
	if lit == nil {
		return nil, nil
	}

	if len(lit.Elts)%3 != 0 {
		return nil, fmt.Errorf("position table has %d entries, want a multiple of 3", len(lit.Elts))
	}
	blocks := make([]coverBlock, 0, len(lit.Elts)/3)
	for i := 0; i < len(lit.Elts); i += 3 {
		startLine, err := parseUintLit(lit.Elts[i])
		if err != nil {
			return nil, err
		}
		endLine, err := parseUintLit(lit.Elts[i+1])
		if err != nil {
			return nil, err
		}
		if startLine == 0 || endLine < startLine {
			return nil, fmt.Errorf("nonsensical block span %d-%d", startLine, endLine)
		}
		blocks = append(blocks, coverBlock{startLine: startLine, endLine: endLine})
	}
	return blocks, nil
}

// findCoverPosLiteral locates the "Pos" element of the coverage variable's
// initializer, or nil when the file carries no instrumentation.
func findCoverPosLiteral(f *ast.File) *ast.CompositeLit {
	for _, decl := range f.Decls {
		genDecl, ok := decl.(*ast.GenDecl)
		if !ok || genDecl.Tok != token.VAR {
			continue
		}
		for _, spec := range genDecl.Specs {
			valueSpec, ok := spec.(*ast.ValueSpec)
			if !ok {
				continue
			}
			for i, name := range valueSpec.Names {
				if name.Name != baselineCoverVar || i >= len(valueSpec.Values) {
					continue
				}
				outer, ok := valueSpec.Values[i].(*ast.CompositeLit)
				if !ok {
					continue
				}
				for _, elt := range outer.Elts {
					kv, ok := elt.(*ast.KeyValueExpr)
					if !ok {
						continue
					}
					key, ok := kv.Key.(*ast.Ident)
					if !ok || key.Name != "Pos" {
						continue
					}
					if inner, ok := kv.Value.(*ast.CompositeLit); ok {
						return inner
					}
				}
			}
		}
	}
	return nil
}

// parseUintLit reads a non-negative integer literal, which cmd/cover emits in
// either decimal or hexadecimal.
func parseUintLit(expr ast.Expr) (int, error) {
	lit, ok := expr.(*ast.BasicLit)
	if !ok || lit.Kind != token.INT {
		return 0, fmt.Errorf("expected an integer literal, got %T", expr)
	}
	v, err := strconv.ParseUint(lit.Value, 0, 32)
	if err != nil {
		return 0, err
	}
	return int(v), nil
}
