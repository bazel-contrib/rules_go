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
	"fmt"
	"go/parser"
	"go/token"
	"io"
	"os"
	"strconv"
	"strings"

	"golang.org/x/tools/go/packages"
)

type ResolvePkgFunc func(importPath string) *packages.Package

// Copy and pasted from golang.org/x/tools/go/packages
type FlatPackagesError struct {
	Pos  string // "file:line:col" or "file:line" or "" or "-"
	Msg  string
	Kind FlatPackagesErrorKind
}

type FlatPackagesErrorKind int

const (
	UnknownError FlatPackagesErrorKind = iota
	ListError
	ParseError
	TypeError
)

func (err FlatPackagesError) Error() string {
	pos := err.Pos
	if pos == "" {
		pos = "-" // like token.Position{}.String()
	}
	return pos + ": " + err.Msg
}

// FlatPackage is the JSON form of Package
// It drops all the type and syntax fields, and transforms the Imports
type FlatPackage struct {
	ID              string
	Name            string              `json:",omitempty"`
	PkgPath         string              `json:",omitempty"`
	Errors          []FlatPackagesError `json:",omitempty"`
	GoFiles         []string            `json:",omitempty"`
	CompiledGoFiles []string            `json:",omitempty"`
	OtherFiles      []string            `json:",omitempty"`
	ExportFile      string              `json:",omitempty"`
	Imports         map[string]string   `json:",omitempty"`
	Standard        bool                `json:",omitempty"`
}

type (
	PackageFunc      func(pkg *FlatPackage)
	PathResolverFunc func(path string) string
)

func resolvePathsInPlace(prf PathResolverFunc, paths []string) {
	for i, path := range paths {
		paths[i] = prf(path)
	}
}

func WalkFlatPackagesFromJSON(jsonFile string, onPkg PackageFunc) error {
	f, err := os.Open(jsonFile)
	if err != nil {
		return fmt.Errorf("unable to open package JSON file: %w", err)
	}
	defer f.Close()

	decoder := json.NewDecoder(f)
	for decoder.More() {
		pkg := &FlatPackage{}
		if err := decoder.Decode(&pkg); err != nil {
			return fmt.Errorf("unable to decode package in %s: %w", f.Name(), err)
		}

		onPkg(pkg)
	}
	return nil
}

func ResolvePaths(pak *packages.Package, prf PathResolverFunc) error {
	resolvePathsInPlace(prf, pak.CompiledGoFiles)
	resolvePathsInPlace(prf, pak.GoFiles)
	resolvePathsInPlace(prf, pak.OtherFiles)
	pak.ExportFile = prf(pak.ExportFile)
	return nil
}

// FilterFilesForBuildTags filters the source files given the current build
// tags.
func FilterFilesForBuildTags(pak *packages.Package) {
	pak.GoFiles = filterSourceFilesForTags(pak.GoFiles)
	pak.CompiledGoFiles = filterSourceFilesForTags(pak.CompiledGoFiles)
}

func filterTestSuffix(pkg *packages.Package, files []string) (err error, testFiles []string, xTestFiles, nonTestFiles []string) {
	for _, filename := range files {
		if strings.HasSuffix(filename, "_test.go") {
			fset := token.NewFileSet()
			f, err := parser.ParseFile(fset, filename, nil, parser.PackageClauseOnly)
			if err != nil {
				return err, nil, nil, nil
			}
			if f.Name.Name == pkg.Name {
				testFiles = append(testFiles, filename)
			} else {
				xTestFiles = append(xTestFiles, filename)
			}
		} else {
			nonTestFiles = append(nonTestFiles, filename)
		}
	}
	return
}

func MoveTestFiles(pkg *packages.Package) *packages.Package {
	err, tgf, xtgf, gf := filterTestSuffix(pkg, pkg.GoFiles)
	if err != nil {
		return nil
	}

	pkg.GoFiles = append(gf, tgf...)

	err, ctgf, cxtgf, cgf := filterTestSuffix(pkg, pkg.CompiledGoFiles)
	if err != nil {
		return nil
	}

	pkg.CompiledGoFiles = append(cgf, ctgf...)

	if len(xtgf) == 0 && len(cxtgf) == 0 {
		return nil
	}

	newImports := make(map[string]*packages.Package, len(pkg.Imports))
	for k, v := range pkg.Imports {
		newImports[k] = v
	}

	newImports[pkg.PkgPath] = &packages.Package{
		ID: pkg.ID,
	}

	// Clone package, only xtgf files
	return &packages.Package{
		ID:              pkg.ID + "_xtest",
		Name:            pkg.Name + "_test",
		PkgPath:         pkg.PkgPath + "_test",
		Imports:         newImports,
		Errors:          pkg.Errors,
		GoFiles:         append([]string{}, xtgf...),
		CompiledGoFiles: append([]string{}, cxtgf...),
		OtherFiles:      pkg.OtherFiles,
		ExportFile:      pkg.ExportFile,
	}
}

func (fp *FlatPackage) IsStdlib() bool {
	return fp.Standard
}

// ResolveImports resolves imports for non-stdlib packages and integrates file overlays
// to allow modification of package imports without modifying disk files.
func ResolveImports(pkg *packages.Package, resolve ResolvePkgFunc, overlays map[string][]byte) error {
	fset := token.NewFileSet()

	for _, file := range pkg.CompiledGoFiles {
		// Only assign overlayContent when an overlay for the file exists, since ParseFile checks by type.
		// If overlay is assigned directly from the map, it will have []byte as type
		// Empty []byte types are parsed into io.EOF
		var overlayReader io.Reader
		if content, ok := overlays[file]; ok {
			overlayReader = bytes.NewReader(content)
		}
		f, err := parser.ParseFile(fset, file, overlayReader, parser.ImportsOnly)
		if err != nil {
			return err
		}
		// If the name is not provided, fetch it from the sources
		if pkg.Name == "" {
			pkg.Name = f.Name.Name
		}

		for _, rawImport := range f.Imports {
			imp, err := strconv.Unquote(rawImport.Path.Value)
			if err != nil {
				continue
			}
			// We don't handle CGo for now
			if imp == "C" {
				continue
			}
			if _, ok := pkg.Imports[imp]; ok {
				continue
			}

			if impPkg := resolve(imp); impPkg != nil {
				pkg.Imports[imp] = impPkg
			}
		}
	}

	return nil
}

func (fp *FlatPackage) IsRoot() bool {
	return strings.HasPrefix(fp.ID, "//")
}
