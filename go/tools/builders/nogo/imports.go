/* Copyright 2018 The Bazel Authors. All rights reserved.

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

package nogo

import (
	"bufio"
	"bytes"
	"encoding/gob"
	"errors"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"go/types"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/tools/go/gcexportdata"
	"golang.org/x/tools/internal/facts"
)

// Adapted from go/src/cmd/compile/internal/gc/main.go. Keep in sync.
func readImportCfg(file string) (packageFile map[string]string, importMap map[string]string, err error) {
	packageFile, importMap = make(map[string]string), make(map[string]string)
	data, err := ioutil.ReadFile(file)
	if err != nil {
		return nil, nil, fmt.Errorf("-importcfg: %v", err)
	}

	for lineNum, line := range strings.Split(string(data), "\n") {
		lineNum++ // 1-based
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		var verb, args string
		if i := strings.Index(line, " "); i < 0 {
			verb = line
		} else {
			verb, args = line[:i], strings.TrimSpace(line[i+1:])
		}
		var before, after string
		if i := strings.Index(args, "="); i >= 0 {
			before, after = args[:i], args[i+1:]
		}
		switch verb {
		default:
			return nil, nil, fmt.Errorf("%s:%d: unknown directive %q", file, lineNum, verb)
		case "importmap":
			if before == "" || after == "" {
				return nil, nil, fmt.Errorf(`%s:%d: invalid importmap: syntax is "importmap old=new"`, file, lineNum)
			}
			importMap[before] = after
		case "packagefile":
			if before == "" || after == "" {
				return nil, nil, fmt.Errorf(`%s:%d: invalid packagefile: syntax is "packagefile path=filename"`, file, lineNum)
			}
			packageFile[before] = after
		}
	}
	return packageFile, importMap, nil
}

// load parses and type checks the source code in each file in filenames.
// load also deserializes facts stored for imported packages.
func load(packagePath, goVersion string, imp *importer, filenames []string, decodeFacts bool) (*goPackage, error) {
	if len(filenames) == 0 {
		return nil, errors.New("no filenames")
	}
	var syntax []*ast.File
	for _, file := range filenames {
		s, err := parser.ParseFile(imp.fset, file, nil, parser.ParseComments)
		if err != nil {
			return nil, err
		}
		syntax = append(syntax, s)
	}
	pkg := &goPackage{fset: imp.fset, syntax: syntax, typesSizes: types.SizesFor("gc", os.Getenv("GOARCH"))}

	config := types.Config{
		GoVersion: goVersion,
		Importer:  imp,
		Sizes:     pkg.typesSizes,
	}
	info := &types.Info{
		Types:      make(map[ast.Expr]types.TypeAndValue),
		Instances:  make(map[*ast.Ident]types.Instance),
		Uses:       make(map[*ast.Ident]types.Object),
		Defs:       make(map[*ast.Ident]types.Object),
		Implicits:  make(map[ast.Node]types.Object),
		Scopes:     make(map[ast.Node]*types.Scope),
		Selections: make(map[*ast.SelectorExpr]*types.Selection),
	}

	initFileVersions(info)

	types, err := config.Check(packagePath, pkg.fset, syntax, info)
	if err != nil {
		pkg.illTyped, pkg.typeCheckError = true, err
	}
	pkg.types, pkg.typesInfo = types, info

	readFacts := imp.readFacts
	if !decodeFacts {
		// Export-only packages do not register or propagate analyzer facts.
		readFacts = func(string) ([]byte, error) { return nil, nil }
	}
	pkg.facts, err = facts.NewDecoder(pkg.types).Decode(readFacts)
	if err != nil {
		return nil, fmt.Errorf("internal error decoding facts: %v", err)
	}

	return pkg, nil
}

// A goPackage describes a loaded Go package.
type goPackage struct {
	// typesSizes is shared by type checking and analysis passes for this invocation.
	typesSizes types.Sizes
	// fset provides position information for types, typesInfo, and syntax.
	// It is set only when types is set.
	fset *token.FileSet
	// syntax is the package's syntax trees.
	syntax []*ast.File
	// types provides type information for the package.
	types *types.Package
	// facts contains information saved by the analysis framework. Passes may
	// import facts for imported packages and may also export facts for this
	// package to be consumed by analyses in downstream packages.
	facts *facts.Set
	// illTyped indicates whether the package or any dependency contains errors.
	// It is set only when types is set.
	illTyped bool
	// typeCheckError contains any error encountered during type-checking. It is
	// only set when illTyped is true.
	typeCheckError error
	// typesInfo provides type information about the package's syntax trees.
	// It is set only when syntax is set.
	typesInfo *types.Info
}

func (g *goPackage) String() string {
	return g.types.Path()
}

// exportData is nogo's private artifact, independent of compiler archives.
// Types is written and read by the same version of x/tools. It includes the
// transitive types reachable from the package's API, so direct inputs suffice.
type exportData struct {
	Types []byte
	Facts []byte
}

// importer imports types and facts produced by nogo, and standard-library
// types obtained through the supported go list -export interface.
type importer struct {
	fset         *token.FileSet
	importMap    map[string]string
	packageCache map[string]*types.Package
	packageFile  map[string]string // standard-library go list -export files
	factMap      map[string]string // canonical package path to nogo export file
	exports      map[string]*exportData
}

func newImporter(importMap, packageFile, factMap map[string]string) *importer {
	return &importer{
		fset:         token.NewFileSet(),
		importMap:    importMap,
		packageCache: make(map[string]*types.Package),
		packageFile:  packageFile,
		factMap:      factMap,
		exports:      make(map[string]*exportData),
	}
}

func (i *importer) readExport(path string) (*exportData, error) {
	if entry, ok := i.exports[path]; ok {
		return entry, nil
	}
	file, ok := i.factMap[path]
	if !ok {
		return nil, nil // standard library: types only, no analysis facts
	}
	f, err := os.Open(file)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	var entry exportData
	if err := gob.NewDecoder(f).Decode(&entry); err != nil {
		return nil, fmt.Errorf("reading nogo export data %s: %v", file, err)
	}
	i.exports[path] = &entry
	return &entry, nil
}

func (i *importer) Import(path string) (*types.Package, error) {
	if imp, ok := i.importMap[path]; ok {
		path = imp
	}
	if path == "unsafe" {
		return types.Unsafe, nil
	}
	if pkg, ok := i.packageCache[path]; ok && pkg.Complete() {
		return pkg, nil
	}
	entry, err := i.readExport(path)
	if err != nil {
		return nil, err
	}
	if entry != nil {
		return gcexportdata.Read(bytes.NewReader(entry.Types), i.fset, i.packageCache, path)
	}

	file, ok := i.packageFile[path]
	if !ok {
		return nil, fmt.Errorf("could not import %q", path)
	}
	f, err := os.Open(file)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	// Older Go releases return an archive from go list -export. New releases
	// return raw indexed data. This compatibility path is only for the public
	// go list interface, never for Bazel's compiler-produced .x files.
	r := bufio.NewReader(f)
	if magic, _ := r.Peek(8); string(magic) == "!<arch>\n" {
		export, err := gcexportdata.NewReader(r)
		if err != nil {
			return nil, fmt.Errorf("reading export data %s: %v", file, err)
		}
		return gcexportdata.Read(export, i.fset, i.packageCache, path)
	}
	return gcexportdata.Read(r, i.fset, i.packageCache, path)
}

func (i *importer) readFacts(pkgPath string) ([]byte, error) {
	entry, err := i.readExport(pkgPath)
	if err != nil || entry == nil {
		return nil, err
	}
	return entry.Facts, nil
}

// writeExport writes types and facts in the format consumed by readExport.
func writeExport(path string, pkg *goPackage) error {
	var typesContent bytes.Buffer
	if err := gcexportdata.Write(&typesContent, pkg.fset, pkg.types); err != nil {
		return fmt.Errorf("error exporting types: %v", err)
	}
	var data bytes.Buffer
	entry := exportData{Types: typesContent.Bytes(), Facts: pkg.facts.Encode()}
	if err := gob.NewEncoder(&data).Encode(entry); err != nil {
		return fmt.Errorf("error encoding export data: %v", err)
	}
	// Absolute paths support long filenames on Windows. Match the bootstrap
	// builder's abs helper, including its macOS wrapper placeholder exemption.
	if !strings.HasPrefix(path, "__BAZEL_") {
		if absolute, err := filepath.Abs(path); err == nil {
			path = absolute
		}
	}
	if err := os.WriteFile(path, data.Bytes(), 0o666); err != nil {
		return fmt.Errorf("error writing export data: %v", err)
	}
	return nil
}
