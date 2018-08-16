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

// compile compiles .go files with "go tool compile". It is invoked by the
// Go rules as an action.
package main

import (
	"bytes"
	"errors"
	"flag"
	"fmt"
	"go/build"
	"io"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"strings"
)

type archive struct {
	importPath, importMap, file string
}

func run(args []string) error {
	// Parse arguments.
	args, err := readParamsFiles(args)
	if err != nil {
		return err
	}
	builderArgs, toolArgs := splitArgs(args)
	flags := flag.NewFlagSet("GoCompile", flag.ExitOnError)
	unfiltered := multiFlag{}
	archives := archiveMultiFlag{}
	goenv := envFlags(flags)
	flags.Var(&unfiltered, "src", "A source file to be filtered and compiled")
	flags.Var(&archives, "arc", "Import path, package path, and file name of a direct dependency, separated by '='")
	output := flags.String("o", "", "The output object file to write")
	packageList := flags.String("package_list", "", "The file containing the list of standard library packages")
	testfilter := flags.String("testfilter", "off", "Controls test package filtering")
	if err := flags.Parse(builderArgs); err != nil {
		return err
	}
	if err := goenv.checkFlags(); err != nil {
		return err
	}
	*output = abs(*output)

	// Filter sources using build constraints.
	var matcher func(f *goMetadata) bool
	switch *testfilter {
	case "off":
		matcher = func(f *goMetadata) bool {
			return true
		}
	case "only":
		matcher = func(f *goMetadata) bool {
			return strings.HasSuffix(f.pkg, "_test")
		}
	case "exclude":
		matcher = func(f *goMetadata) bool {
			return !strings.HasSuffix(f.pkg, "_test")
		}
	default:
		return fmt.Errorf("Invalid test filter %q", *testfilter)
	}
	// apply build constraints to the source list
	all, err := readFiles(build.Default, unfiltered)
	if err != nil {
		return err
	}
	files := []*goMetadata{}
	for _, f := range all {
		if matcher(f) {
			files = append(files, f)
		}
	}
	if len(files) == 0 {
		// We need to run the compiler to create a valid archive, even if there's
		// nothing in it. GoPack will complain if we try to add assembly or cgo
		// objects.
		emptyPath := filepath.Join(filepath.Dir(*output), "_empty.go")
		if err := ioutil.WriteFile(emptyPath, []byte("package empty\n"), 0666); err != nil {
			return err
		}
		files = append(files, &goMetadata{filename: emptyPath})
	}

	// Check that the filtered sources don't import anything outside of
	// the standard library and the direct dependencies.
	_, stdImports, err := checkDirectDeps(files, archives, *packageList)
	if err != nil {
		return err
	}

	// Build an importcfg file for the compiler.
	importcfgName, err := buildImportcfgFile(archives, stdImports, goenv.installSuffix, filepath.Dir(*output))
	if err != nil {
		return err
	}
	defer os.Remove(importcfgName)

	// Compile the filtered files.
	goargs := goenv.goTool("compile")
	goargs = append(goargs, "-importcfg", importcfgName)
	goargs = append(goargs, "-pack", "-o", *output)
	goargs = append(goargs, toolArgs...)
	goargs = append(goargs, "--")
	for _, f := range files {
		goargs = append(goargs, f.filename)
	}
	absArgs(goargs, []string{"-I", "-o", "-trimpath", "-importcfg"})
	return goenv.runCommand(goargs)
}

func main() {
	log.SetFlags(0) // no timestamp
	log.SetPrefix("GoCompile: ")
	if err := run(os.Args[1:]); err != nil {
		log.Fatal(err)
	}
}

func checkDirectDeps(files []*goMetadata, archives []archive, packageList string) (depImports, stdImports []string, err error) {
	packagesTxt, err := ioutil.ReadFile(packageList)
	if err != nil {
		log.Fatal(err)
	}
	stdlibSet := map[string]bool{}
	for _, line := range strings.Split(string(packagesTxt), "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			stdlibSet[line] = true
		}
	}

	depSet := map[string]bool{}
	depList := make([]string, len(archives))
	for i, arc := range archives {
		depSet[arc.importPath] = true
		depList[i] = arc.importPath
	}

	importSet := map[string]bool{}

	derr := depsError{known: depList}
	for _, f := range files {
		for _, path := range f.imports {
			if path == "C" || isRelative(path) || importSet[path] {
				// TODO(#1645): Support local (relative) import paths. We don't emit
				// errors for them here, but they will probably break something else.
				continue
			}
			if stdlibSet[path] {
				stdImports = append(stdImports, path)
				continue
			}
			if depSet[path] {
				depImports = append(depImports, path)
				continue
			}
			derr.missing = append(derr.missing, missingDep{f.filename, path})
		}
	}
	if len(derr.missing) > 0 {
		return nil, nil, derr
	}
	return depImports, stdImports, nil
}

func buildImportcfgFile(archives []archive, stdImports []string, installSuffix, dir string) (string, error) {
	buf := &bytes.Buffer{}
	goroot, ok := os.LookupEnv("GOROOT")
	if !ok {
		return "", errors.New("GOROOT not set")
	}
	goroot = abs(goroot)
	for _, imp := range stdImports {
		path := filepath.Join(goroot, "pkg", installSuffix, filepath.FromSlash(imp))
		fmt.Fprintf(buf, "packagefile %s=%s.a\n", imp, path)
	}
	for _, arc := range archives {
		if arc.importPath != arc.importMap {
			fmt.Fprintf(buf, "importmap %s=%s\n", arc.importPath, arc.importMap)
		}
		fmt.Fprintf(buf, "packagefile %s=%s\n", arc.importMap, arc.file)
	}
	f, err := ioutil.TempFile(dir, "importcfg")
	if err != nil {
		return "", err
	}
	filename := f.Name()
	if _, err := io.Copy(f, buf); err != nil {
		f.Close()
		os.Remove(filename)
		return "", err
	}
	if err := f.Close(); err != nil {
		os.Remove(filename)
		return "", err
	}
	return filename, nil
}

type archiveMultiFlag []archive

func (m *archiveMultiFlag) String() string {
	if m == nil || len(*m) == 0 {
		return ""
	}
	return fmt.Sprint(*m)
}

func (m *archiveMultiFlag) Set(v string) error {
	parts := strings.Split(v, "=")
	if len(parts) != 3 {
		return fmt.Errorf("badly formed -arc flag: %s", v)
	}
	*m = append(*m, archive{
		importPath: parts[0],
		importMap:  parts[1],
		file:       abs(parts[2]),
	})
	return nil
}

type depsError struct {
	missing []missingDep
	known   []string
}

type missingDep struct {
	filename, imp string
}

var _ error = depsError{}

func (e depsError) Error() string {
	buf := bytes.NewBuffer(nil)
	fmt.Fprintf(buf, "missing strict dependencies:\n")
	for _, dep := range e.missing {
		fmt.Fprintf(buf, "\t%s: import of %q\n", dep.filename, dep.imp)
	}
	if len(e.known) == 0 {
		fmt.Fprintln(buf, "No dependencies were provided.")
	} else {
		fmt.Fprintln(buf, "Known dependencies are:")
		for _, imp := range e.known {
			fmt.Fprintf(buf, "\t%s\n", imp)
		}
	}
	fmt.Fprint(buf, "Check that imports in Go sources match importpath attributes in deps.")
	return buf.String()
}

func isRelative(path string) bool {
	return strings.HasPrefix(path, "./") || strings.HasPrefix(path, "../")
}
