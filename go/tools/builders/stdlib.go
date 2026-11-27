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
	"flag"
	"fmt"
	"go/build"
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
	out := flags.String("out", "", "Path to output go root")
	race := flags.Bool("race", false, "Build in race mode")
	msan := flags.Bool("msan", false, "Build in msan mode")
	shared := flags.Bool("shared", false, "Build in shared mode")
	dynlink := flags.Bool("dynlink", false, "Build in dynlink mode")
	pgoprofile := flags.String("pgoprofile", "", "Build with pgo using the given pprof file")
	var packages multiFlag
	flags.Var(&packages, "package", "Packages to build")
	var gcflags quoteMultiFlag
	flags.Var(&gcflags, "gcflags", "Go compiler flags")
	var fipsPackageList string
	flags.StringVar(&fipsPackageList, "fips_package_list", "", "The file containing the list of packages in the GOFIPS140 snapshot")
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

	// Fail fast if cgo is required but a toolchain is not configured.
	if os.Getenv("CGO_ENABLED") == "1" && filepath.Base(os.Getenv("CC")) == "vc_installation_error.bat" {
		return fmt.Errorf(`cgo is required, but a C toolchain has not been configured.
You may need to use the flags --cpu=x64_windows --compiler=mingw-gcc.`)
	}

	// Link in the bare minimum needed to the new GOROOT
	if err := replicate(goroot, output, replicatePaths(append([]string{"src", "pkg/tool", "pkg/include"}, fipsGorootPaths()...)...)); err != nil {
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

	// The go command unpacks a GOFIPS140 snapshot into the module cache, which
	// is read-only without -modcacherw and could then not be removed.
	if fipsSnapshot() {
		modCachePath := filepath.Join(output, ".gomodcache")
		os.Setenv("GOMODCACHE", modCachePath)
		os.Setenv("GOFLAGS", "-modcacherw")
		defer os.RemoveAll(modCachePath)
	}

	// Make sure we have an absolute path to the C compiler.
	cc := abs(os.Getenv("CC"))
	os.Setenv("CC", quotePathIfNeeded(cc))

	// Ensure paths are absolute.
	absPaths := []string{}
	for _, path := range filepath.SplitList(os.Getenv("PATH")) {
		absPaths = append(absPaths, abs(path))
	}
	os.Setenv("PATH", strings.Join(absPaths, string(os.PathListSeparator)))

	sandboxPath := abs(".")

	// Strip path prefix from source files in debug information.
	os.Setenv("CGO_CFLAGS", os.Getenv("CGO_CFLAGS")+" "+strings.Join(defaultCFlags(output, cc), " "))
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
	buildFlags := []string{"-toolexec", abs(os.Args[0]) + " filterbuildid"}
	if len(build.Default.BuildTags) > 0 {
		buildFlags = append(buildFlags, "-tags", strings.Join(build.Default.BuildTags, ","))
	}

	ldflags := []string{"-trimpath", sandboxPath}
	asmflags := []string{"-trimpath", output}
	if *race {
		buildFlags = append(buildFlags, "-race")
	}
	if *msan {
		buildFlags = append(buildFlags, "-msan")
	}
	if *pgoprofile != "" {
		gcflags = append(gcflags, "-pgoprofile=" + abs(*pgoprofile))
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

	buildFlags = append(buildFlags, "-gcflags=all="+strings.Join(gcflags, " "))
	buildFlags = append(buildFlags, "-ldflags=all="+strings.Join(ldflags, " "))
	buildFlags = append(buildFlags, "-asmflags=all="+strings.Join(asmflags, " "))

	if err := absCCCompiler(cgoEnvVars, cgoAbsEnvFlags); err != nil {
		return fmt.Errorf("error modifying cgo environment to absolute path: %v", err)
	}

	installArgs := goenv.goCmd("install")
	installArgs = append(installArgs, buildFlags...)
	installArgs = append(installArgs, packages...)
	if err := goenv.runCommand(installArgs); err != nil {
		return err
	}
	if fipsSnapshot() {
		return installFIPSSnapshotArchives(goenv, output, buildFlags, *race, *msan, fipsPackageList)
	}
	return nil
}

// installFIPSSnapshotArchives places the archives of the GOFIPS140 snapshot
// packages into goroot/pkg. "go install std" compiles them but leaves them in
// the build cache (golang/go#76225), where the linker cannot find them.
// Building each one with the flags "go install std" used is a cache hit that
// copies the archive out.
func installFIPSSnapshotArchives(goenv *env, goroot string, buildFlags []string, race, msan bool, packageListPath string) error {
	pkgs, err := readFIPSPackageList(packageListPath)
	if err != nil {
		return err
	}
	if len(pkgs) == 0 {
		return fmt.Errorf("GOFIPS140=%s: %s lists no snapshot packages", os.Getenv("GOFIPS140"), packageListPath)
	}
	platform := os.Getenv("GOOS") + "_" + os.Getenv("GOARCH")
	if race {
		platform += "_race"
	} else if msan {
		platform += "_msan"
	}
	for _, pkg := range pkgs {
		dst := filepath.Join(goroot, "pkg", platform, filepath.FromSlash(pkg)+".a")
		if _, err := os.Stat(dst); err == nil {
			continue
		}
		if err := os.MkdirAll(filepath.Dir(dst), 0o777); err != nil {
			return err
		}
		buildArgs := goenv.goCmd("build")
		buildArgs = append(buildArgs, buildFlags...)
		buildArgs = append(buildArgs, "-o", dst, pkg)
		if err := goenv.runCommand(buildArgs); err != nil {
			return err
		}
	}
	return nil
}
