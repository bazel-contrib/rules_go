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
	"bytes"
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"go/parser"
	"go/token"
	"strconv"
)

const writeFileMode = 0o666

// instrumentForCoverage runs "go tool cover" on a source file to produce
// a coverage-instrumented version of the file. It also registers the file
// with the coverdata package.
func instrumentForCoverage(
		goenv *env,
		importPath string,
		pkgName string,
		infiles []string,
		coverVar string,
		coverMode string,
		outfiles []string,
		workDir string,
		relCoverPath map[string]string,
		srcPathMapping map[string]string,
	) ([]string, error) {
	// This implementation follows the go toolchain's setup of the pkgcfg file
	// https://github.com/golang/go/blob/go1.24.5/src/cmd/go/internal/work/exec.go#L1954
	pkgcfg := workDir + "pkgcfg.txt"
	covoutputs := workDir + "coveroutfiles.txt"
	odir := filepath.Dir(outfiles[0])
	cv := filepath.Join(odir, "covervars.go")
	outputFiles := append([]string{cv}, outfiles...)

	pcfg := coverPkgConfig{
		PkgPath:   importPath,
		PkgName:   pkgName,
		Granularity: "perblock",
		OutConfig: pkgcfg,
		Local:     false,
	}
	data, err := json.Marshal(pcfg)
	if err != nil {
		return nil, err
	}
	data = append(data, '\n')
	if err := os.WriteFile(pkgcfg, data, writeFileMode); err != nil {
		return nil, err
	}
	var sb strings.Builder
	for i := range outputFiles {
		fmt.Fprintf(&sb, "%s\n", outputFiles[i])
	}
	if err := os.WriteFile(covoutputs, []byte(sb.String()), writeFileMode); err != nil {
		return nil, err
	}

	goargs := goenv.goTool("cover", "-pkgcfg", pkgcfg, "-var", coverVar, "-mode", coverMode, "-outfilelist", covoutputs)
	goargs = append(goargs, infiles...)
	if err := goenv.runCommand(goargs); err != nil {
		return nil, err
	}

	for i, outfile := range outfiles {
		srcName := relCoverPath[infiles[i]]
		importPathFile := srcPathMapping[srcName]
		// Augment coverage source files to store a mapping of <importpath>/<filename> -> <execroot_relative_path>
		// as this information is only known during compilation but is required when the rules_go generated
		// test main exits and go coverage files are converted to lcov format.
		if err := registerCoverage(outfile, importPathFile, srcName); err != nil {
			return nil, err
		}
	}
	return outputFiles, nil
}

// coverPkgConfig matches https://cs.opensource.google/go/go/+/refs/tags/go1.24.4:src/cmd/internal/cov/covcmd/cmddefs.go;l=18
type coverPkgConfig struct {
	// File into which cmd/cover should emit summary info
	// when instrumentation is complete.
	OutConfig string

	// Import path for the package being instrumented.
	PkgPath string

	// Package name.
	PkgName string

	// Instrumentation granularity: one of "perfunc" or "perblock" (default)
	Granularity string

	// Module path for this package (empty if no go.mod in use)
	ModulePath string

	// Local mode indicates we're doing a coverage build or test of a
	// package selected via local import path, e.g. "./..." or
	// "./foo/bar" as opposed to a non-relative import path. See the
	// corresponding field in cmd/go's PackageInternal struct for more
	// info.
	Local bool

	// EmitMetaFile if non-empty is the path to which the cover tool should
	// directly emit a coverage meta-data file for the package, if the
	// package has any functions in it. The go command will pass in a value
	// here if we've been asked to run "go test -cover" on a package that
	// doesn't have any *_test.go files.
	EmitMetaFile string
}

// registerCoverage modifies coverSrcFilename, the output file from go tool cover.
// It adds a call to coverdata.RegisterSrcPathMapping, which ensures that rules_go
// can produce lcov files with exec root relative file paths.
func registerCoverage(coverSrcFilename, importPathFile, srcName string) error {
	coverSrc, err := os.ReadFile(coverSrcFilename)
	if err != nil {
		return fmt.Errorf("instrumentForCoverage: reading instrumented source: %w", err)
	}

	// Parse the file.
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, coverSrcFilename, coverSrc, parser.ParseComments)
	if err != nil {
		return nil // parse error: proceed and let the compiler fail
	}

	// Perform edits using a byte buffer instead of the AST, because
	// we can not use go/format to write the AST back out without
	// changing line numbers.
	editor := NewBuffer(coverSrc)

	// Ensure coverdata is imported. Use an existing import if present
	// or add a new one.
	const coverdataPath = "github.com/bazelbuild/rules_go/go/tools/coverdata"
	var coverdataName string
	for _, imp := range f.Imports {
		path, err := strconv.Unquote(imp.Path.Value)
		if err != nil {
			return nil // parse error: proceed and let the compiler fail
		}
		if path == coverdataPath {
			if imp.Name != nil {
				// renaming import
				if imp.Name.Name == "_" {
					// Change blank import to named import
					editor.Replace(
						fset.Position(imp.Name.Pos()).Offset,
						fset.Position(imp.Name.End()).Offset,
						"coverdata")
					coverdataName = "coverdata"
				} else {
					coverdataName = imp.Name.Name
				}
			} else {
				// default import
				coverdataName = "coverdata"
			}
			break
		}
	}
	if coverdataName == "" {
		// No existing import. Add a new one.
		coverdataName = "coverdata"
		editor.Insert(fset.Position(f.Name.End()).Offset, fmt.Sprintf("; import %q", coverdataPath))
	}

	// Append an init function.
	var buf = bytes.NewBuffer(editor.Bytes())
	fmt.Fprintf(buf, `
func init() {
	%s.RegisterSrcPathMapping(%q, %q)
}
`, coverdataName, importPathFile, srcName)
	if err := os.WriteFile(coverSrcFilename, buf.Bytes(), writeFileMode); err != nil {
		return fmt.Errorf("registerCoverage: %v", err)
	}
	return nil
}

// coveragePath returns the location path of coverage counter data emitted by
// the go runtime. With the go coverage redesign, the location path looks to be
// importpath + the file base name. Line directives are honored but the location
// path is still importpath relative.
func coveragePath(src string, importPath string) (string, error) {
	directiveName, err := findFirstLineDirectiveFilename(src)
	if err != nil {
		return "", err
	}
	filename := src
	if directiveName != "" {
		fmt.Println("directiveName: ", directiveName)
		filename = directiveName
	}
	return importPath + "/" + filepath.Base(filename), nil
}

// lineDirective represents a parsed line directive
type lineDirective struct {
	Filename string
	Line     int
	Column   int // 0 if not specified
}

// parseLineDirective extracts information from a Go line directive.
// It handles both //line and /*line*/ formats and validates the results.
func parseLineDirective(directive string) (*lineDirective, error) {
	// Remove leading/trailing whitespace
	directive = strings.TrimSpace(directive)

	var content string

	// Handle //line directive
	if strings.HasPrefix(directive, "//line ") {
		content = directive[7:] // Remove "//line "
	} else if strings.HasPrefix(directive, "/*line ") && strings.HasSuffix(directive, "*/") {
		// Handle /*line*/ directive
		content = directive[7 : len(directive)-2] // Remove "/*line " and "*/"
	} else {
		return nil, fmt.Errorf("not a valid line directive")
	}

	if content == "" {
		return nil, fmt.Errorf("empty line directive content")
	}

	return parseLineContent(content)
}

// parseLineContent parses the content after "line " prefix
func parseLineContent(content string) (*lineDirective, error) {
	// Use a more sophisticated approach to handle Windows paths
	// Find all colons and work backwards to find line/column numbers
	parts := strings.Split(content, ":")
	if len(parts) < 2 {
		return nil, fmt.Errorf("missing line number")
	}

	// Try to parse as filename:line:col
	if len(parts) >= 3 {
		// Check if last two parts are valid numbers
		colStr := parts[len(parts)-1]
		lineStr := parts[len(parts)-2]

		col, colErr := strconv.Atoi(colStr)
		line, lineErr := strconv.Atoi(lineStr)

		if colErr == nil && lineErr == nil && line > 0 && col > 0 {
			// Valid filename:line:col format
			filename := strings.Join(parts[:len(parts)-2], ":")
			return &lineDirective{
				Filename: filename,
				Line:     line,
				Column:   col,
			}, nil
		}
	}

	// Try to parse as filename:line
	lineStr := parts[len(parts)-1]
	line, err := strconv.Atoi(lineStr)
	if err != nil || line <= 0 {
		return nil, fmt.Errorf("invalid line number: %s", lineStr)
	}

	filename := strings.Join(parts[:len(parts)-1], ":")

	return &lineDirective{
		Filename: filename,
		Line:     line,
		Column:   0,
	}, nil
}

// findFirstLineDirectiveFilename opens a Go file, scans it, and returns the
// filename from the first line directive it finds.
func findFirstLineDirectiveFilename(filePath string) (string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return "", fmt.Errorf("open file: %w", err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		if ld, err := parseLineDirective(line); err == nil && ld != nil {
			return ld.Filename, nil // Found the first one, return it.
		}
	}
	if err := scanner.Err(); err != nil {
		return "", fmt.Errorf("scanning file: %w", err)
	}
	// no line directive found, return empty string
	return "", nil
}
