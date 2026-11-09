// Copyright 2018 The Bazel Authors. All rights reserved.
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
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// stdlib builds the standard library in the appropriate mode into a new goroot.
func stdlib(args []string) error {
	// process the args
	flags := flag.NewFlagSet("stdlib", flag.ExitOnError)
	goenv := envFlags(flags)
	out := flags.String("out", "", "Path to output go root or analysis export directory")
	export := flags.Bool("export", false, "Copy go list export data for analysis instead of installing archives")
	race := flags.Bool("race", false, "Build in race mode")
	msan := flags.Bool("msan", false, "Build in msan mode")
	shared := flags.Bool("shared", false, "Build in shared mode")
	dynlink := flags.Bool("dynlink", false, "Build in dynlink mode")
	pgoprofile := flags.String("pgoprofile", "", "Build with pgo using the given pprof file")
	var packages multiFlag
	flags.Var(&packages, "package", "Packages to build")
	var gcflags quoteMultiFlag
	flags.Var(&gcflags, "gcflags", "Go compiler flags")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if err := goenv.checkFlagsAndSetGoroot(); err != nil {
		return err
	}
	goroot := os.Getenv("GOROOT")
	if goroot == "" {
		return fmt.Errorf("GOROOT not set")
	}
	output := abs(*out)
	if *export {
		work, cleanup, err := goenv.workDir()
		if err != nil {
			return err
		}
		defer cleanup()
		output = filepath.Join(work, "goroot")
	}

	// Fail fast if cgo is required but a toolchain is not configured.
	if os.Getenv("CGO_ENABLED") == "1" && filepath.Base(os.Getenv("CC")) == "vc_installation_error.bat" {
		return fmt.Errorf(`cgo is required, but a C toolchain has not been configured.
You may need to use the flags --cpu=x64_windows --compiler=mingw-gcc.`)
	}

	// Link in the bare minimum needed to the new GOROOT
	if err := replicate(goroot, output, replicatePaths("src", "pkg/tool", "pkg/include")); err != nil {
		return err
	}

	output, err := processPath(output)
	if err != nil {
		return err
	}

	// Now switch to the newly created GOROOT
	os.Setenv("GOROOT", output)

	// Create a temporary cache directory for "go build".
	cachePath := filepath.Join(output, ".gocache")
	os.Setenv("GOCACHE", cachePath)
	defer os.RemoveAll(cachePath)

	// Disable modules for the 'go install' command. Depending on the sandboxing
	// mode, there may be a go.mod file in a parent directory which will turn
	// modules on in "auto" mode.
	os.Setenv("GO111MODULE", "off")

	// Make sure we have an absolute path to the C compiler.
	os.Setenv("CC", quotePathIfNeeded(abs(os.Getenv("CC"))))

	// Ensure paths are absolute.
	absPaths := []string{}
	for _, path := range filepath.SplitList(os.Getenv("PATH")) {
		absPaths = append(absPaths, abs(path))
	}
	os.Setenv("PATH", strings.Join(absPaths, string(os.PathListSeparator)))

	sandboxPath := abs(".")

	// Strip path prefix from source files in debug information.
	cflags := defaultCFlags(output)
	if *export {
		// go list -trimpath supplies its own source path mappings. Explicit
		// mappings contain our temporary GOROOT and would make the cache's
		// build IDs (and thus exported archives) nondeterministic.
		var nonPathFlags []string
		for _, f := range cflags {
			if !strings.HasPrefix(f, "-ffile-prefix-map=") {
				nonPathFlags = append(nonPathFlags, f)
			}
		}
		cflags = nonPathFlags
	}
	os.Setenv("CGO_CFLAGS", os.Getenv("CGO_CFLAGS")+" "+strings.Join(cflags, " "))
	os.Setenv("CGO_LDFLAGS", os.Getenv("CGO_LDFLAGS")+" "+strings.Join(defaultLdFlags(), " "))

	// Allow flags in CGO_LDFLAGS that wouldn't pass the security check.
	// Workaround for golang.org/issue/42565.
	var b strings.Builder
	sep := ""
	cgoLdflags, _ := splitQuoted(os.Getenv("CGO_LDFLAGS"))
	for _, f := range cgoLdflags {
		b.WriteString(sep)
		sep = "|"
		b.WriteString(regexp.QuoteMeta(f))
		// If the flag if -framework, the flag value needs to be in the same
		// condition.
		if f == "-framework" {
			sep = " "
		}
	}
	os.Setenv("CGO_LDFLAGS_ALLOW", b.String())
	os.Setenv("GODEBUG", "installgoroot=all")

	// Build the commands needed to build the std library in the right mode
	// NOTE: the go command stamps compiled .a files with build ids, which are
	// cryptographic sums derived from the inputs. This prevents us from
	// creating reproducible builds because the build ids are hashed from
	// CGO_CFLAGS, which frequently contains absolute paths. As a workaround,
	// we strip the build ids, since they won't be used after this.
	installArgs := goenv.goCmd("install", "-toolexec", abs(os.Args[0])+" filterbuildid")
	if *export {
		// go list needs build IDs to populate Export from its cache. Unlike
		// install, it must not use filterbuildid. Trim source paths instead.
		installArgs = goenv.goCmd("list", "-export", "-deps", "-json", "-trimpath")
	}
	if len(build.Default.BuildTags) > 0 {
		installArgs = append(installArgs, "-tags", strings.Join(build.Default.BuildTags, ","))
	}

	ldflags := []string{"-trimpath", sandboxPath}
	asmflags := []string{"-trimpath", output}
	if *export {
		// These absolute paths also enter go list's cache keys. Let its
		// -trimpath option provide the compiler and assembler mappings.
		ldflags = nil
		asmflags = nil
	}
	if *race {
		installArgs = append(installArgs, "-race")
	}
	if *msan {
		installArgs = append(installArgs, "-msan")
	}
	if *pgoprofile != "" {
		gcflags = append(gcflags, "-pgoprofile="+abs(*pgoprofile))
	}
	if *shared {
		gcflags = append(gcflags, "-shared")
		ldflags = append(ldflags, "-shared")
		asmflags = append(asmflags, "-shared")
	}
	if *dynlink {
		gcflags = append(gcflags, "-dynlink")
		ldflags = append(ldflags, "-dynlink")
		asmflags = append(asmflags, "-dynlink")
	}

	installArgs = append(installArgs, "-gcflags=all="+strings.Join(gcflags, " "))
	installArgs = append(installArgs, "-ldflags=all="+strings.Join(ldflags, " "))
	installArgs = append(installArgs, "-asmflags=all="+strings.Join(asmflags, " "))

	if err := absCCCompiler(cgoEnvVars, cgoAbsEnvFlags); err != nil {
		return fmt.Errorf("error modifying cgo environment to absolute path: %v", err)
	}
	if *export {
		// The Go cache hashes CC verbatim. Resolve the builder through PATH
		// so that sandbox-specific executable paths do not enter build IDs.
		// GO_CC still identifies the actual compiler for the cc wrapper.
		builder := abs(os.Args[0])
		os.Setenv("PATH", filepath.Dir(builder)+string(os.PathListSeparator)+os.Getenv("PATH"))
		os.Setenv("CC", quotePathIfNeeded(filepath.Base(builder))+" cc")
	}

	installArgs = append(installArgs, packages...)
	if *export {
		return exportStdlib(goenv, installArgs, abs(*out))
	}
	if err := goenv.runCommand(installArgs); err != nil {
		return err
	}
	return nil
}

// exportStdlib copies the files advertised by go list's supported Export API.
// Older SDKs return archives, while newer SDKs may return analysis-specific
// export data. Keep them opaque here: the analysis importer handles both forms.
func exportStdlib(goenv *env, args []string, out string) error {
	var data bytes.Buffer
	if err := goenv.runCommandToFile(&data, os.Stderr, args); err != nil {
		return err
	}
	if err := os.MkdirAll(out, 0777); err != nil {
		return err
	}
	decoder := json.NewDecoder(&data)
	for {
		var pkg struct {
			ImportPath string
			Export     string
			GoFiles    []string
			CgoFiles   []string
		}
		if err := decoder.Decode(&pkg); err == io.EOF {
			return nil
		} else if err != nil {
			return fmt.Errorf("decoding standard library export metadata: %v", err)
		}
		// unsafe is synthesized by go/types. Test-only packages also have
		// no export data, although they may appear in the std pattern.
		if pkg.ImportPath == "unsafe" || (len(pkg.GoFiles) == 0 && len(pkg.CgoFiles) == 0) {
			continue
		}
		if pkg.Export == "" {
			return fmt.Errorf("no export data for standard library package %q", pkg.ImportPath)
		}
		dst := filepath.Join(out, filepath.FromSlash(pkg.ImportPath)+".x")
		if err := os.MkdirAll(filepath.Dir(dst), 0777); err != nil {
			return err
		}
		if err := copyFile(pkg.Export, dst); err != nil {
			return fmt.Errorf("copying export data for %q: %v", pkg.ImportPath, err)
		}
	}
}
