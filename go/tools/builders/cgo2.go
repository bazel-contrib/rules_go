// Copyright 2019 The Bazel Authors. All rights reserved.
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

// cgo2.go provides new cgo functionality for use by the GoCompilePkg action.
// We can't use the functionality in cgo.go, since it relies too heavily
// on logic in cgo.bzl. Ideally, we'd be able to replace cgo.go with this
// file eventually, but not until Bazel gives us enough toolchain information
// to compile ObjC files.

package main

import (
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// cgo2 processes a set of mixed source files with cgo.
func cgo2(goenv *env, goSrcs, cgoSrcs, cSrcs, cxxSrcs, objcSrcs, objcxxSrcs, sSrcs, hSrcs []string, packagePath, packageName string, cc string, cppFlags, cFlags, cxxFlags, objcFlags, objcxxFlags, ldFlags []string, cgoExportHPath string, cgoGoSrcsPath string) (srcDir string, allGoSrcs, cObjs []string, err error) {
	// Report an error if the C/C++ toolchain wasn't configured.
	if cc == "" {
		err := cgoError(cgoSrcs[:])
		err = append(err, cSrcs...)
		err = append(err, cxxSrcs...)
		err = append(err, objcSrcs...)
		err = append(err, objcxxSrcs...)
		err = append(err, sSrcs...)
		return "", nil, nil, err
	}

	// If we only have C/C++ sources without cgo, just compile and pack them
	// without generating code. The Go command forbids this, but we've
	// historically allowed it.
	// TODO(jayconrod): this doesn't write CGO_LDFLAGS into the archive. We
	// might miss dependencies like -lstdc++ if they aren't referenced in
	// some other way.
	if len(cgoSrcs) == 0 {
		cObjs, err = compileCSources(goenv, cSrcs, cxxSrcs, objcSrcs, objcxxSrcs, sSrcs, hSrcs, cc, cppFlags, cFlags, cxxFlags, objcFlags, objcxxFlags)
		return ".", nil, cObjs, err
	}

	workDir, cleanup, err := goenv.workDir()
	if err != nil {
		return "", nil, nil, err
	}
	defer cleanup()

	// cgo2 will gather sources into a single temporary directory, since nogo
	// scanners might want to include or exclude these sources we need to ensure
	// that a fragment of the path is stable and human friendly enough to be
	// referenced in nogo configuration.
	workDir = filepath.Join(workDir, "cgo", packagePath)
	if err := os.MkdirAll(workDir, 0700); err != nil {
		return "", nil, nil, err
	}

	// Filter out -lstdc++ and -lc++ from ldflags if we don't have C++ sources,
	// and set CGO_LDFLAGS. These flags get written as special comments into cgo
	// generated sources. The compiler encodes those flags in the compiled .a
	// file, and the linker passes them on to the external linker.
	haveCxx := len(cxxSrcs)+len(objcxxSrcs) > 0
	if !haveCxx {
		for _, f := range ldFlags {
			if strings.HasSuffix(f, ".a") {
				// These flags come from cdeps options. Assume C++.
				haveCxx = true
				break
			}
		}
	}
	var combinedLdFlags []string
	if haveCxx {
		combinedLdFlags = append(combinedLdFlags, ldFlags...)
	} else {
		for _, f := range ldFlags {
			if f != "-lc++" && f != "-lstdc++" {
				combinedLdFlags = append(combinedLdFlags, f)
			}
		}
	}
	combinedLdFlags = append(combinedLdFlags, defaultLdFlags()...)
	os.Setenv("CGO_LDFLAGS", strings.Join(combinedLdFlags, " "))

	// If cgo sources are in different directories, gather them into a temporary
	// directory so we can use -srcdir.
	srcDir = filepath.Dir(cgoSrcs[0])
	srcsInSingleDir := true
	for _, src := range cgoSrcs[1:] {
		if filepath.Dir(src) != srcDir {
			srcsInSingleDir = false
			break
		}
	}

	if srcsInSingleDir {
		for i := range cgoSrcs {
			cgoSrcs[i] = filepath.Base(cgoSrcs[i])
		}
	} else {
		srcDir = filepath.Join(workDir, "cgosrcs")
		if err := os.Mkdir(srcDir, 0777); err != nil {
			return "", nil, nil, err
		}
		copiedSrcs, err := gatherSrcs(srcDir, cgoSrcs)
		if err != nil {
			return "", nil, nil, err
		}
		cgoSrcs = copiedSrcs
	}

	// Generate Go and C code.
	hdrDirs := map[string]bool{}
	var hdrIncludes []string
	for _, hdr := range hSrcs {
		hdrDir := filepath.Dir(hdr)
		if !hdrDirs[hdrDir] {
			hdrDirs[hdrDir] = true
			hdrIncludes = append(hdrIncludes, "-iquote", hdrDir)
		}
	}
	hdrIncludes = append(hdrIncludes, "-iquote", workDir) // for _cgo_export.h

	execRoot, err := bazelExecRoot()
	if err != nil {
		return "", nil, nil, err
	}
	// Trim the execroot from the //line comments emitted by cgo.
	args := goenv.goTool("cgo", "-srcdir", srcDir, "-objdir", workDir, "-trimpath", execRoot)
	if packagePath != "" {
		args = append(args, "-importpath", packagePath)
	}
	args = append(args, "--")
	args = append(args, cppFlags...)
	args = append(args, hdrIncludes...)
	args = append(args, cFlags...)
	args = append(args, cgoSrcs...)
	if err := goenv.runCommand(args); err != nil {
		return "", nil, nil, err
	}

	if cgoExportHPath != "" {
		if err := copyFile(filepath.Join(workDir, "_cgo_export.h"), cgoExportHPath); err != nil {
			return "", nil, nil, err
		}
	}
	genGoSrcs := make([]string, 1+len(cgoSrcs))
	genGoSrcs[0] = filepath.Join(workDir, "_cgo_gotypes.go")
	genCSrcs := make([]string, 1+len(cgoSrcs))
	genCSrcs[0] = filepath.Join(workDir, "_cgo_export.c")
	for i, src := range cgoSrcs {
		stem := strings.TrimSuffix(filepath.Base(src), ".go")
		genGoSrcs[i+1] = filepath.Join(workDir, stem+".cgo1.go")
		genCSrcs[i+1] = filepath.Join(workDir, stem+".cgo2.c")
	}
	cgoMainC := filepath.Join(workDir, "_cgo_main.c")

	// Compile C, C++, Objective-C/C++, and assembly code.
	defaultCFlags := defaultCFlags(workDir)
	combinedCFlags := combineFlags(cppFlags, hdrIncludes, cFlags, defaultCFlags)
	for _, lang := range []struct{ srcs, flags []string }{
		{genCSrcs, combinedCFlags},
		{cSrcs, combinedCFlags},
		{cxxSrcs, combineFlags(cppFlags, hdrIncludes, cxxFlags, defaultCFlags)},
		{objcSrcs, combineFlags(cppFlags, hdrIncludes, objcFlags, defaultCFlags)},
		{objcxxSrcs, combineFlags(cppFlags, hdrIncludes, objcxxFlags, defaultCFlags)},
		{sSrcs, nil},
	} {
		for _, src := range lang.srcs {
			obj := filepath.Join(workDir, fmt.Sprintf("_x%d.o", len(cObjs)))
			cObjs = append(cObjs, obj)
			if err := cCompile(goenv, src, cc, lang.flags, obj); err != nil {
				return "", nil, nil, err
			}
		}
	}

	mainObj := filepath.Join(workDir, "_cgo_main.o")
	if err := cCompile(goenv, cgoMainC, cc, combinedCFlags, mainObj); err != nil {
		return "", nil, nil, err
	}

	// Link cgo binary and use the symbols to generate _cgo_import.go.
	mainBin := filepath.Join(workDir, "_cgo_.o") // .o is a lie; it's an executable
	args = append([]string{cc, "-o", mainBin, mainObj}, cObjs...)
	args = append(args, combinedLdFlags...)
	var originalErrBuf bytes.Buffer
	if err := goenv.runCommandToFile(os.Stdout, &originalErrBuf, args); err != nil {
		// If linking the binary for cgo fails, this is usually because the
		// object files reference external symbols that can't be resolved yet.
		// Since the binary is only produced to have its symbols read by the cgo
		// command, there is no harm in trying to build it allowing unresolved
		// symbols - the real link that happens at the end will fail if they
		// rightfully can't be resolved.
		var allowUnresolvedSymbolsLdFlag string
		switch os.Getenv("GOOS") {
		case "windows":
			// MinGW's linker doesn't seem to support --unresolved-symbols
			// and MSVC isn't supported at all.
			return "", nil, nil, err
		case "darwin", "ios":
			allowUnresolvedSymbolsLdFlag = "-Wl,-undefined,dynamic_lookup"
		default:
			allowUnresolvedSymbolsLdFlag = "-Wl,--unresolved-symbols=ignore-all"
		}
		// Print and return the original error if we can't link the binary with
		// the additional linker flags as they may simply be incorrect for the
		// particular compiler/linker pair and would obscure the true reason for
		// the failure of the original command.
		if err2 := goenv.runCommandToFile(
			os.Stdout,
			ioutil.Discard,
			append(args, allowUnresolvedSymbolsLdFlag),
		); err2 != nil {
			os.Stderr.Write(relativizePaths(originalErrBuf.Bytes()))
			return "", nil, nil, err
		}
		// Do not print the original error - rerunning the command with the
		// additional linker flag fixed it.
	}

	cgoImportsGo := filepath.Join(workDir, "_cgo_imports.go")
	args = goenv.goTool("cgo", "-dynpackage", packageName, "-dynimport", mainBin, "-dynout", cgoImportsGo)
	if err := goenv.runCommand(args); err != nil {
		return "", nil, nil, err
	}
	genGoSrcs = append(genGoSrcs, cgoImportsGo)
	if cgoGoSrcsPath != "" {
		for _, src := range genGoSrcs {
			if err := copyFile(src, filepath.Join(cgoGoSrcsPath, filepath.Base(src))); err != nil {
				return "", nil, nil, err
			}
		}
	}

	// Copy regular Go source files into the work directory so that we can
	// use -trimpath=workDir.
	goBases, err := gatherSrcs(workDir, goSrcs)
	if err != nil {
		return "", nil, nil, err
	}

	allGoSrcs = make([]string, len(goSrcs)+len(genGoSrcs))
	for i := range goSrcs {
		allGoSrcs[i] = filepath.Join(workDir, goBases[i])
	}
	copy(allGoSrcs[len(goSrcs):], genGoSrcs)
	return workDir, allGoSrcs, cObjs, nil
}

// compileCSources compiles a list of C, C++, Objective-C, Objective-C++,
// and assembly sources into .o files to be packed into the archive.
// It does not run cgo. This is used for packages with "cgo = True" but
// without any .go files that import "C". The Go command forbids this,
// but we have historically allowed it.
func compileCSources(goenv *env, cSrcs, cxxSrcs, objcSrcs, objcxxSrcs, sSrcs, hSrcs []string, cc string, cppFlags, cFlags, cxxFlags, objcFlags, objcxxFlags []string) (cObjs []string, err error) {
	workDir, cleanup, err := goenv.workDir()
	if err != nil {
		return nil, err
	}
	defer cleanup()

	hdrDirs := map[string]bool{}
	var hdrIncludes []string
	for _, hdr := range hSrcs {
		hdrDir := filepath.Dir(hdr)
		if !hdrDirs[hdrDir] {
			hdrDirs[hdrDir] = true
			hdrIncludes = append(hdrIncludes, "-iquote", hdrDir)
		}
	}

	defaultCFlags := defaultCFlags(workDir)
	for _, lang := range []struct{ srcs, flags []string }{
		{cSrcs, combineFlags(cppFlags, hdrIncludes, cFlags, defaultCFlags)},
		{cxxSrcs, combineFlags(cppFlags, hdrIncludes, cxxFlags, defaultCFlags)},
		{objcSrcs, combineFlags(cppFlags, hdrIncludes, objcFlags, defaultCFlags)},
		{objcxxSrcs, combineFlags(cppFlags, hdrIncludes, objcxxFlags, defaultCFlags)},
		{sSrcs, nil},
	} {
		for _, src := range lang.srcs {
			obj := filepath.Join(workDir, fmt.Sprintf("_x%d.o", len(cObjs)))
			cObjs = append(cObjs, obj)
			if err := cCompile(goenv, src, cc, lang.flags, obj); err != nil {
				return nil, err
			}
		}
	}
	return cObjs, nil
}

func combineFlags(lists ...[]string) []string {
	n := 0
	for _, list := range lists {
		n += len(list)
	}
	flags := make([]string, 0, n)
	for _, list := range lists {
		flags = append(flags, list...)
	}
	return flags
}

func cCompile(goenv *env, src, cc string, flags []string, out string) error {
	args := []string{cc}
	args = append(args, flags...)
	args = append(args, "-c", src, "-o", out)
	return goenv.runCommand(args)
}

func defaultCFlags(workDir string) []string {
	flags := []string{
		"-fdebug-prefix-map=" + abs(".") + "=.",
		"-fdebug-prefix-map=" + workDir + "=.",
		"-fcoverage-prefix-map=" + abs(".") + "=.",
		"-fcoverage-prefix-map=" + workDir + "=.",
		"-fmacro-prefix-map=" + abs(".") + "=.",
		"-fmacro-prefix-map=" + workDir + "=.",
	}
	goos, goarch := os.Getenv("GOOS"), os.Getenv("GOARCH")
	switch {
	case goos == "darwin" || goos == "ios":
		return flags
	case goos == "windows" && goarch == "amd64":
		return append(flags, "-mthreads")
	default:
		return append(flags, "-pthread")
	}
}

func defaultLdFlags() []string {
	goos, goarch := os.Getenv("GOOS"), os.Getenv("GOARCH")
	switch {
	case goos == "android":
		return []string{"-llog", "-ldl"}
	case goos == "darwin" || goos == "ios":
		return nil
	case goos == "windows" && goarch == "amd64":
		return []string{"-mthreads"}
	default:
		return []string{"-pthread"}
	}
}

// gatherSrcs copies or links files listed in srcs into dir. This is needed
// to effectively use -trimpath with generated sources. It's also needed by cgo.
//
// gatherSrcs returns the basenames of copied files in the directory.
func gatherSrcs(dir string, srcs []string) ([]string, error) {
	copiedBases := make([]string, len(srcs))
	for i, src := range srcs {
		base := filepath.Base(src)
		ext := filepath.Ext(base)
		stem := base[:len(base)-len(ext)]
		var err error
		for j := 1; j < 10000; j++ {
			if err = copyOrLinkFile(src, filepath.Join(dir, base)); err == nil {
				break
			} else if !os.IsExist(err) {
				return nil, err
			} else {
				base = fmt.Sprintf("%s_%d%s", stem, j, ext)
			}
		}
		if err != nil {
			return nil, fmt.Errorf("could not find unique name for file %s", src)
		}
		copiedBases[i] = base
	}
	return copiedBases, nil
}

func bazelExecRoot() (string, error) {
	// Bazel executes the builder with a working directory of the form
	// .../execroot/<workspace name>. By stripping the last segment, we obtain a
	// prefix of all possible source files, even when contained in external
	// repositories.
	cwd, err := os.Getwd()
	if err != nil {
		return "", err
	}
	return filepath.Dir(cwd), nil
}

type cgoError []string

func (e cgoError) Error() string {
	b := &bytes.Buffer{}
	fmt.Fprint(b, "CC is not set and files need to be processed with cgo:\n")
	for _, f := range e {
		fmt.Fprintf(b, "\t%s\n", f)
	}
	fmt.Fprintf(b, "Ensure that 'cgo = True' is set and the C/C++ toolchain is configured.")
	return b.String()
}

func copyFile(inPath, outPath string) error {
	inFile, err := os.Open(inPath)
	if err != nil {
		return err
	}
	defer inFile.Close()
	outFile, err := os.OpenFile(outPath, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0666)
	if err != nil {
		return err
	}
	defer outFile.Close()
	_, err = io.Copy(outFile, inFile)
	return err
}

func linkFile(inPath, outPath string) error {
	inPath, err := filepath.Abs(inPath)
	if err != nil {
		return err
	}
	return os.Symlink(inPath, outPath)
}

func copyOrLinkFile(inPath, outPath string) error {
	if runtime.GOOS == "windows" {
		return copyFile(inPath, outPath)
	} else {
		return linkFile(inPath, outPath)
	}
}
