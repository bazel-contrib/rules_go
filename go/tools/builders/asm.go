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
	"go/build"
	"os"
	"path/filepath"
)

var ASM_DEFINES = []string{
	"-D", "GOOS_" + build.Default.GOOS,
	"-D", "GOARCH_" + build.Default.GOARCH,
	"-D", "GOOS_GOARCH_" + build.Default.GOOS + "_" + build.Default.GOARCH,
}

// buildSymabisFile generates a file from assembly files that is consumed by
// the compiler. If there are no assembly files, no file will be generated,
// and "", nil will be returned.
func buildSymabisFile(goenv *env, packagePath string, sFiles, hFiles []fileInfo, asmhdr string) (string, error) {
	if len(sFiles) == 0 {
		return "", nil
	}

	// Create an empty go_asm.h file. The compiler will write this later, but
	// we need one to exist now.
	asmhdrFile, err := os.Create(asmhdr)
	if err != nil {
		return "", err
	}
	if err := asmhdrFile.Close(); err != nil {
		return "", err
	}
	asmhdrDir := filepath.Dir(asmhdr)

	// Create a temporary output file. The caller is responsible for deleting it.
	var symabisName string
	symabisFile, err := os.CreateTemp("", "symabis")
	if err != nil {
		return "", err
	}
	symabisName = symabisFile.Name()
	symabisFile.Close()

	// Run the assembler.
	wd, err := os.Getwd()
	if err != nil {
		return symabisName, err
	}
	asmargs := goenv.goTool("asm")
	asmargs = append(asmargs, "-trimpath", wd)
	asmargs = append(asmargs, "-I", wd)
	asmargs = append(asmargs, "-I", filepath.Join(os.Getenv("GOROOT"), "pkg", "include"))
	asmargs = append(asmargs, "-I", asmhdrDir)
	seenHdrDirs := map[string]bool{wd: true, asmhdrDir: true}
	for _, hFile := range hFiles {
		hdrDir := filepath.Dir(abs(hFile.filename))
		if !seenHdrDirs[hdrDir] {
			asmargs = append(asmargs, "-I", hdrDir)
			seenHdrDirs[hdrDir] = true
		}
	}
	// Go emits -p when preparing symabis so that the package path is available
	// to the assembler.
	if packagePath != "" {
		asmargs = append(asmargs, "-p", packagePath)
	}
	asmargs = append(asmargs, ASM_DEFINES...)
	asmargs = append(asmargs, "-gensymabis", "-o", symabisName, "--")
	for _, sFile := range sFiles {
		asmargs = append(asmargs, sFile.filename)
	}

	err = goenv.runCommand(asmargs)
	return symabisName, err
}

func asmFile(goenv *env, srcPath, packagePath string, asmFlags []string, outPath string) error {
	args := goenv.goTool("asm")
	args = append(args, asmFlags...)
	if packagePath != "" {
		args = append(args, "-p", packagePath)
	}
	args = append(args, ASM_DEFINES...)
	args = append(args, "-trimpath", ".")
	args = append(args, "-o", outPath)
	args = append(args, "--", srcPath)
	absArgs(args, []string{"-I", "-o", "-trimpath"})
	return goenv.runCommand(args)
}
