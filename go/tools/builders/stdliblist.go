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
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"go/build"
	"os"
	"path/filepath"
	"strings"
)

// Copy and pasted from golang.org/x/tools/go/packages
type flatPackagesError struct {
	Pos  string // "file:line:col" or "file:line" or "" or "-"
	Msg  string
	Kind flatPackagesErrorKind
}

type flatPackagesErrorKind int

const (
	UnknownError flatPackagesErrorKind = iota
	ListError
	ParseError
	TypeError
)

func (err flatPackagesError) Error() string {
	pos := err.Pos
	if pos == "" {
		pos = "-" // like token.Position{}.String()
	}
	return pos + ": " + err.Msg
}

// flatPackage is the JSON form of Package
// It drops all the type and syntax fields, and transforms the Imports
type flatPackage struct {
	ID              string
	Name            string              `json:",omitempty"`
	PkgPath         string              `json:",omitempty"`
	Standard        bool                `json:",omitempty"`
	Errors          []flatPackagesError `json:",omitempty"`
	GoFiles         []string            `json:",omitempty"`
	CompiledGoFiles []string            `json:",omitempty"`
	OtherFiles      []string            `json:",omitempty"`
	ExportFile      string              `json:",omitempty"`
	Imports         map[string]string   `json:",omitempty"`
}

type goListPackage struct {
	Dir        string // directory containing package sources
	ImportPath string // import path of package in dir
	Name       string // package name
	Target     string // install path
	Goroot     bool   // is this package in the Go root?
	Standard   bool   // is this package part of the standard Go library?
	Root       string // Go root or Go path dir containing this package
	Export     string // file containing export data (when using -export)
	// Source files
	GoFiles           []string // .go source files (excluding CgoFiles, TestGoFiles, XTestGoFiles)
	CgoFiles          []string // .go source files that import "C"
	CompiledGoFiles   []string // .go files presented to compiler (when using -compiled)
	IgnoredGoFiles    []string // .go source files ignored due to build constraints
	IgnoredOtherFiles []string // non-.go source files ignored due to build constraints
	CFiles            []string // .c source files
	CXXFiles          []string // .cc, .cxx and .cpp source files
	MFiles            []string // .m source files
	HFiles            []string // .h, .hh, .hpp and .hxx source files
	FFiles            []string // .f, .F, .for and .f90 Fortran source files
	SFiles            []string // .s source files
	SwigFiles         []string // .swig files
	SwigCXXFiles      []string // .swigcxx files
	SysoFiles         []string // .syso object files to add to archive
	TestGoFiles       []string // _test.go files in package
	XTestGoFiles      []string // _test.go files outside package
	// Embedded files
	EmbedPatterns      []string // //go:embed patterns
	EmbedFiles         []string // files matched by EmbedPatterns
	TestEmbedPatterns  []string // //go:embed patterns in TestGoFiles
	TestEmbedFiles     []string // files matched by TestEmbedPatterns
	XTestEmbedPatterns []string // //go:embed patterns in XTestGoFiles
	XTestEmbedFiles    []string // files matched by XTestEmbedPatterns
	// Dependency information
	Imports   []string          // import paths used by this package
	ImportMap map[string]string // map from source import to ImportPath (identity entries omitted)
	// Error information
	Incomplete bool                 // this package or a dependency has an error
	Error      *flatPackagesError   // error loading package
	DepsErrors []*flatPackagesError // errors loading dependencies
}

var rulesGoStdlibPrefix string

func init() {
	if rulesGoStdlibPrefix == "" {
		panic("rulesGoStdlibPrefix should have been set via -X")
	}
}

func stdlibPackageID(importPath string) string {
	return rulesGoStdlibPrefix + importPath
}

// outputBasePath replace the cloneBase with output base label
func outputBasePath(cloneBase, p string) string {
	dir, _ := filepath.Rel(cloneBase, p)
	return filepath.Join("__BAZEL_OUTPUT_BASE__", dir)
}

// absoluteSourcesPaths replace cloneBase of the absolution
// paths with the label for all source files in a package
func absoluteSourcesPaths(cloneBase, pkgDir string, srcs []string) []string {
	ret := make([]string, 0, len(srcs))
	pkgDir = outputBasePath(cloneBase, pkgDir)
	for _, src := range srcs {
		absPath := src

		// Generated files will already have an absolute path. These come from
		// the compiler's cache.
		if !filepath.IsAbs(src) {
			absPath = filepath.Join(pkgDir, src)
		}

		ret = append(ret, absPath)
	}
	return ret
}

// filterGoFiles keeps only files either ending in .go or those without an
// extension (which are from the cache). This is a work around for
// https://golang.org/issue/28749: cmd/go puts assembly, C, and C++ files in
// CompiledGoFiles.
func filterGoFiles(srcs []string, pathReplaceFn func(p string) string) []string {
	ret := make([]string, 0, len(srcs))
	for _, f := range srcs {
		if ext := filepath.Ext(f); ext == ".go" || ext == "" {
			ret = append(ret, pathReplaceFn(f))
		}
	}

	return ret
}

func flatPackageForStd(cloneBase string, pkg *goListPackage, pathReplaceFn func(p string) string) *flatPackage {
	goFiles := absoluteSourcesPaths(cloneBase, pkg.Dir, pkg.GoFiles)
	compiledGoFiles := absoluteSourcesPaths(cloneBase, pkg.Dir, pkg.CompiledGoFiles)

	newPkg := &flatPackage{
		ID:              stdlibPackageID(pkg.ImportPath),
		Name:            pkg.Name,
		PkgPath:         pkg.ImportPath,
		ExportFile:      outputBasePath(cloneBase, pkg.Target),
		Imports:         map[string]string{},
		Standard:        pkg.Standard,
		GoFiles:         goFiles,
		CompiledGoFiles: filterGoFiles(compiledGoFiles, pathReplaceFn),
	}

	// imports
	//
	// Imports contains the IDs of all imported packages.
	// ImportsMap records (path, ID) only where they differ.
	ids := make(map[string]struct{})
	for _, id := range pkg.Imports {
		ids[id] = struct{}{}
	}

	for path, id := range pkg.ImportMap {
		newPkg.Imports[path] = stdlibPackageID(id)
		delete(ids, id)
	}

	for id := range ids {
		if id != "C" {
			newPkg.Imports[id] = stdlibPackageID(id)
		}
	}

	return newPkg
}

// stdliblist runs `go list -json` on the standard library and saves it to a file.
func stdliblist(args []string) error {
	// process the args
	flags := flag.NewFlagSet("stdliblist", flag.ExitOnError)
	goenv := envFlags(flags)
	out := flags.String("out", "", "Path to output go list json")
	cachePath := flags.String("cache", "", "Path to use for GOCACHE")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if err := goenv.checkFlagsAndSetGoroot(); err != nil {
		return err
	}

	if filepath.IsAbs(goenv.sdk) {
		return fmt.Errorf("-sdk needs to be a relative path, but got %s", goenv.sdk)
	}

	// In Go 1.18, the standard library started using go:embed directives.
	// When Bazel runs this action, it does so inside a sandbox where GOROOT points
	// to an external/go_sdk directory that contains a symlink farm of all files in
	// the Go SDK.
	// If we run "go list" with that GOROOT, this action will fail because those
	// go:embed directives will refuse to include the symlinks in the sandbox.
	//
	// To work around this, cloneGoRoot creates a copy of a subset of external/go_sdk
	// that is sufficient to call "go list" into a new cloneBase directory, e.g.
	// "go list" needs to call "compile", which needs "pkg/tool".
	// We also need to retain the same relative path to the root directory, e.g.
	// "$OUTPUT_BASE/external/go_sdk" becomes
	// {cloneBase}/external/go_sdk", which will be set at GOROOT later. This ensures
	// that file paths in the generated JSON are still valid.
	//
	// Here we replicate goRoot(absolute path of goenv.sdk) to newGoRoot.
	cloneBase, cleanup, err := goenv.workDir()
	if err != nil {
		return err
	}
	defer func() { cleanup() }()

	newGoRoot := filepath.Join(cloneBase, goenv.sdk)
	if err := replicate(abs(goenv.sdk), abs(newGoRoot), replicatePaths("src", "pkg/tool", "pkg/include")); err != nil {
		return err
	}

	// Ensure paths are absolute.
	absPaths := []string{}
	for _, path := range filepath.SplitList(os.Getenv("PATH")) {
		absPaths = append(absPaths, abs(path))
	}
	os.Setenv("PATH", strings.Join(absPaths, string(os.PathListSeparator)))
	os.Setenv("GOROOT", newGoRoot)

	cgoEnabled := os.Getenv("CGO_ENABLED") == "1"
	// Make sure we have an absolute path to the C compiler.
	ccEnv, ok := os.LookupEnv("CC")
	if cgoEnabled && !ok {
		return fmt.Errorf("CC must be set")
	}
	os.Setenv("CC", quotePathIfNeeded(abs(ccEnv)))

	if err := absCCCompiler(cgoEnvVars, cgoAbsEnvFlags); err != nil {
		return fmt.Errorf("error modifying cgo environment to absolute path: %v", err)
	}

	// We want to keep the cache around so that the processed files can be used by other tools.
	absCachePath := abs(*cachePath)
	os.Setenv("GOCACHE", absCachePath)
	os.Setenv("GOMODCACHE", absCachePath)
	os.Setenv("GOPATH", absCachePath)

	listArgs := goenv.goCmd("list")
	if len(build.Default.BuildTags) > 0 {
		listArgs = append(listArgs, "-tags", strings.Join(build.Default.BuildTags, ","))
	}

	listArgs = append(listArgs, "-compiled=true")
	listArgs = append(listArgs, "-json", "builtin", "std", "runtime/cgo")

	jsonFile, err := os.Create(*out)
	if err != nil {
		return err
	}
	defer jsonFile.Close()

	jsonData := &bytes.Buffer{}
	if err := goenv.runCommandToFile(jsonData, os.Stderr, listArgs); err != nil {
		return err
	}

	encoder := json.NewEncoder(jsonFile)
	decoder := json.NewDecoder(jsonData)
	pathReplaceFn := func(s string) string {
		if strings.HasPrefix(s, absCachePath) {
			return strings.Replace(s, absCachePath, filepath.Join("__BAZEL_EXECROOT__", *cachePath), 1)
		}

		return s
	}
	for decoder.More() {
		var pkg *goListPackage
		if err := decoder.Decode(&pkg); err != nil {
			return err
		}
		if err := encoder.Encode(flatPackageForStd(cloneBase, pkg, pathReplaceFn)); err != nil {
			return err
		}
	}

	return nil
}
