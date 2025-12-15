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

package main

import (
	"fmt"
	"go/ast"
	"go/build"
	"go/token"
	"os"
	"path/filepath"
	"strings"
)

type fileInfo struct {
	filename string
	ext      ext
	header   []byte
	fset     *token.FileSet
	parsed   *ast.File
	parseErr error
	matched  bool
	isCgo    bool
	pkg      string
	imports  []fileImport
	embeds   []fileEmbed
}

type ext int

const (
	goExt ext = iota
	cExt
	cxxExt
	objcExt
	objcxxExt
	sExt
	hExt
	sysoExt
)

type fileImport struct {
	path string
	pos  token.Pos
	doc  *ast.CommentGroup
}

type fileEmbed struct {
	pattern string
	pos     token.Position
}

type archiveSrcs struct {
	goSrcs, cSrcs, cxxSrcs, objcSrcs, objcxxSrcs, sSrcs, hSrcs, sysoSrcs []fileInfo
}

// filterAndSplitFiles filters files using build constraints and collates
// them by extension.
func filterAndSplitFiles(fileNames []string) (archiveSrcs, error) {
	var res archiveSrcs
	packageContainsCgo := false
	for _, s := range fileNames {
		src, err := readFileInfo(build.Default, s)
		if err != nil {
			return archiveSrcs{}, err
		}
		if src.isCgo {
			packageContainsCgo = true
		}
		if !src.matched {
			continue
		}
		var srcs *[]fileInfo
		switch src.ext {
		case goExt:
			srcs = &res.goSrcs
		case cExt:
			srcs = &res.cSrcs
		case cxxExt:
			srcs = &res.cxxSrcs
		case objcExt:
			srcs = &res.objcSrcs
		case objcxxExt:
			srcs = &res.objcxxSrcs
		case sExt:
			srcs = &res.sSrcs
		case hExt:
			srcs = &res.hSrcs
		case sysoExt:
			srcs = &res.sysoSrcs
		}
		*srcs = append(*srcs, src)
	}
	if packageContainsCgo && !build.Default.CgoEnabled {
		// Cgo packages use the C compiler for asm files, rather than Go's assembler.
		// This is a package with cgo files, but we are compiling with Cgo disabled:
		// Remove the assembly files.
		res.sSrcs = nil
	}
	return res, nil
}

// applyTestFilter filters out test files from the list of sources in place
// according to the filter.
func applyTestFilter(testFilter string, srcs *archiveSrcs) error {
	// TODO(jayconrod): remove -testfilter flag. The test action should compile
	// the main, internal, and external packages by calling compileArchive
	// with the correct sources for each.
	switch testFilter {
	case "off":
	case "only":
		testSrcs := make([]fileInfo, 0, len(srcs.goSrcs))
		for _, f := range srcs.goSrcs {
			if strings.HasSuffix(f.pkg, "_test") {
				testSrcs = append(testSrcs, f)
			}
		}
		srcs.goSrcs = testSrcs
	case "exclude":
		libSrcs := make([]fileInfo, 0, len(srcs.goSrcs))
		for _, f := range srcs.goSrcs {
			if !strings.HasSuffix(f.pkg, "_test") {
				libSrcs = append(libSrcs, f)
			}
		}
		srcs.goSrcs = libSrcs
	default:
		return fmt.Errorf("invalid test filter %q", testFilter)
	}
	return nil
}

// readFileInfo applies build constraints to an input file and returns whether
// it should be compiled.
func readFileInfo(bctx build.Context, input string) (fileInfo, error) {
	fi := fileInfo{filename: input}
	if ext := filepath.Ext(input); ext == ".C" {
		fi.ext = cxxExt
	} else {
		switch strings.ToLower(ext) {
		case ".go":
			fi.ext = goExt
		case ".c":
			fi.ext = cExt
		case ".cc", ".cxx", ".cpp":
			fi.ext = cxxExt
		case ".m":
			fi.ext = objcExt
		case ".mm":
			fi.ext = objcxxExt
		case ".s":
			fi.ext = sExt
		case ".h", ".hh", ".hpp", ".hxx":
			fi.ext = hExt
		case ".syso":
			fi.ext = sysoExt
		default:
			return fileInfo{}, fmt.Errorf("unrecognized file extension: %s", ext)
		}
	}

	dir, base := filepath.Split(input)
	// Check build constraints on non-cgo files.
	// Skip cgo files, since they get rejected (due to leading '_') and won't
	// have any build constraints anyway.
	if strings.HasPrefix(base, "_cgo") {
		fi.matched = true
	} else {
		match, err := bctx.MatchFile(dir, base)
		if err != nil {
			return fi, err
		}
		fi.matched = match
	}
	// If it's not a go file, there's nothing more to read.
	if fi.ext != goExt {
		return fi, nil
	}

	// Scan the file for imports and embeds.
	f, err := os.Open(input)
	if err != nil {
		return fileInfo{}, err
	}
	defer f.Close()
	fi.fset = token.NewFileSet()
	if err := readGoInfo(f, &fi); err != nil {
		return fileInfo{}, err
	}

	// Exclude cgo files if cgo is not enabled.
	for _, imp := range fi.imports {
		if imp.path == "C" {
			fi.isCgo = true
			break
		}
	}
	fi.matched = fi.matched && (bctx.CgoEnabled || !fi.isCgo)

	return fi, nil
}
