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

// compilepkg compiles a complete Go package from Go, C, and assembly files.  It
// supports cgo, coverage, and nogo. It is invoked by the Go rules as an action.
package main

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
)

func compilePkg(args []string) error {
	// Parse arguments.
	args, _, err := expandParamsFiles(args)
	if err != nil {
		return err
	}

	fs := flag.NewFlagSet("GoCompilePkg", flag.ExitOnError)
	goenv := envFlags(fs)
	var unfilteredSrcs, coverSrcs, embedSrcs, embedLookupDirs, embedRoots, recompileInternalDeps multiFlag
	var deps archiveMultiFlag
	var importPath, packagePath, packageListPath, coverMode string
	var outLinkobjPath, outInterfacePath, cgoExportHPath, cgoGoSrcsPath string
	var testFilter string
	var gcFlags, asmFlags, cppFlags, cFlags, cxxFlags, objcFlags, objcxxFlags, ldFlags quoteMultiFlag
	var coverFormat string
	var pgoprofile string
	fs.Var(&unfilteredSrcs, "src", ".go, .c, .cc, .m, .mm, .s, or .S file to be filtered and compiled")
	fs.Var(&coverSrcs, "cover", ".go file that should be instrumented for coverage (must also be a -src)")
	fs.Var(&embedSrcs, "embedsrc", "file that may be compiled into the package with a //go:embed directive")
	fs.Var(&embedLookupDirs, "embedlookupdir", "Root-relative paths to directories relative to which //go:embed directives are resolved")
	fs.Var(&embedRoots, "embedroot", "Bazel output root under which a file passed via -embedsrc resides")
	fs.Var(&deps, "arc", "Import path, package path, and file name of a direct dependency, separated by '='")
	fs.StringVar(&importPath, "importpath", "", "The import path of the package being compiled. Not passed to the compiler, but may be displayed in debug data.")
	fs.StringVar(&packagePath, "p", "", "The package path (importmap) of the package being compiled")
	fs.Var(&gcFlags, "gcflags", "Go compiler flags")
	fs.Var(&asmFlags, "asmflags", "Go assembler flags")
	fs.Var(&cppFlags, "cppflags", "C preprocessor flags")
	fs.Var(&cFlags, "cflags", "C compiler flags")
	fs.Var(&cxxFlags, "cxxflags", "C++ compiler flags")
	fs.Var(&objcFlags, "objcflags", "Objective-C compiler flags")
	fs.Var(&objcxxFlags, "objcxxflags", "Objective-C++ compiler flags")
	fs.Var(&ldFlags, "ldflags", "C linker flags")
	fs.StringVar(&packageListPath, "package_list", "", "The file containing the list of standard library packages")
	fs.StringVar(&coverMode, "cover_mode", "", "The coverage mode to use. Empty if coverage instrumentation should not be added.")
	fs.StringVar(&outLinkobjPath, "lo", "", "The full output archive file required by the linker")
	fs.StringVar(&outInterfacePath, "o", "", "The export-only output archive required to compile dependent packages")
	fs.StringVar(&cgoExportHPath, "cgoexport", "", "The _cgo_exports.h file to write")
	fs.StringVar(&cgoGoSrcsPath, "cgo_go_srcs", "", "The directory to emit cgo-generated Go sources for nogo consumption to")
	fs.StringVar(&testFilter, "testfilter", "off", "Controls test package filtering")
	fs.StringVar(&coverFormat, "cover_format", "", "Emit source file paths in coverage instrumentation suitable for the specified coverage format")
	fs.Var(&recompileInternalDeps, "recompile_internal_deps", "The import path of the direct dependencies that needs to be recompiled.")
	fs.StringVar(&pgoprofile, "pgoprofile", "", "The pprof profile to consider for profile guided optimization.")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if err := goenv.checkFlagsAndSetGoroot(); err != nil {
		return err
	}
	if importPath == "" {
		importPath = packagePath
	}
	cgoEnabled := os.Getenv("CGO_ENABLED") == "1"
	cc := os.Getenv("CC")
	outLinkobjPath = abs(outLinkobjPath)
	for i := range unfilteredSrcs {
		unfilteredSrcs[i] = abs(unfilteredSrcs[i])
	}
	for i := range embedSrcs {
		embedSrcs[i] = abs(embedSrcs[i])
	}
	if pgoprofile != "" {
		pgoprofile = abs(pgoprofile)
	}

	// Filter sources.
	srcs, err := filterAndSplitFiles(unfilteredSrcs)
	if err != nil {
		return err
	}

	err = applyTestFilter(testFilter, &srcs)
	if err != nil {
		return err
	}

	return compileArchive(
		goenv,
		importPath,
		packagePath,
		srcs,
		deps,
		coverMode,
		coverSrcs,
		embedSrcs,
		embedLookupDirs,
		embedRoots,
		cgoEnabled,
		cc,
		gcFlags,
		asmFlags,
		cppFlags,
		cFlags,
		cxxFlags,
		objcFlags,
		objcxxFlags,
		ldFlags,
		packageListPath,
		outLinkobjPath,
		outInterfacePath,
		cgoExportHPath,
		cgoGoSrcsPath,
		coverFormat,
		recompileInternalDeps,
		pgoprofile)
}

func compileArchive(
	goenv *env,
	importPath string,
	packagePath string,
	srcs archiveSrcs,
	deps []archive,
	coverMode string,
	coverSrcs []string,
	embedSrcs []string,
	embedLookupDirs []string,
	embedRoots []string,
	cgoEnabled bool,
	cc string,
	gcFlags []string,
	asmFlags []string,
	cppFlags []string,
	cFlags []string,
	cxxFlags []string,
	objcFlags []string,
	objcxxFlags []string,
	ldFlags []string,
	packageListPath string,
	outLinkObj string,
	outInterfacePath string,
	cgoExportHPath string,
	cgoGoSrcsForNogoPath string,
	coverFormat string,
	recompileInternalDeps []string,
	pgoprofile string,
) error {
	workDir, cleanup, err := goenv.workDir()
	if err != nil {
		return err
	}
	defer cleanup()

	if len(srcs.goSrcs) == 0 {
		// We need to run the compiler to create a valid archive, even if there's nothing in it.
		// Otherwise, GoPack will complain if we try to add assembly or cgo objects.
		// A truly empty archive does not include any references to source file paths, which
		// ensures hermeticity even though the temp file path is random.
		emptyGoFile, err := os.CreateTemp(filepath.Dir(outLinkObj), "*.go")
		if err != nil {
			return err
		}
		defer os.Remove(emptyGoFile.Name())
		defer emptyGoFile.Close()
		if _, err := emptyGoFile.WriteString("package empty\n"); err != nil {
			return err
		}
		if err := emptyGoFile.Close(); err != nil {
			return err
		}

		srcs.goSrcs = append(srcs.goSrcs, fileInfo{
			filename: emptyGoFile.Name(),
			ext:      goExt,
			matched:  true,
			pkg:      "empty",
		})
	}
	packageName := srcs.goSrcs[0].pkg
	var goSrcs, cgoSrcs []string
	for _, src := range srcs.goSrcs {
		if src.isCgo {
			cgoSrcs = append(cgoSrcs, src.filename)
		} else {
			goSrcs = append(goSrcs, src.filename)
		}
	}
	cSrcs := make([]string, len(srcs.cSrcs))
	for i, src := range srcs.cSrcs {
		cSrcs[i] = src.filename
	}
	cxxSrcs := make([]string, len(srcs.cxxSrcs))
	for i, src := range srcs.cxxSrcs {
		cxxSrcs[i] = src.filename
	}
	objcSrcs := make([]string, len(srcs.objcSrcs))
	for i, src := range srcs.objcSrcs {
		objcSrcs[i] = src.filename
	}
	objcxxSrcs := make([]string, len(srcs.objcxxSrcs))
	for i, src := range srcs.objcxxSrcs {
		objcxxSrcs[i] = src.filename
	}
	sSrcs := make([]string, len(srcs.sSrcs))
	for i, src := range srcs.sSrcs {
		sSrcs[i] = src.filename
	}
	hSrcs := make([]string, len(srcs.hSrcs))
	for i, src := range srcs.hSrcs {
		hSrcs[i] = src.filename
	}

	// haveCgo is true if the package contains Cgo files.
	haveCgo := len(cgoSrcs)+len(cSrcs)+len(cxxSrcs)+len(objcSrcs)+len(objcxxSrcs) > 0
	// compilingWithCgo is true if the package contains Cgo files AND Cgo is enabled. A package
	// containing Cgo files can also be built with Cgo disabled, and will work if there are build
	// constraints.
	compilingWithCgo := haveCgo && cgoEnabled

	// When coverage is set, source files will be modified during instrumentation. We should only run static analysis
	// over original source files and not the modified ones.
	// goSrcsNogo and cgoSrcsNogo are copies of the original source files for nogo to run static analysis.
	// TODO: Use slices.Clone when 1.21 is the minimal supported version.
	goSrcsNogo := append([]string{}, goSrcs...)
	cgoSrcsNogo := append([]string{}, cgoSrcs...)

	// Instrument source files for coverage.
	if coverMode != "" {
		relCoverPath := make(map[string]string)
		for _, s := range coverSrcs {
			relCoverPath[abs(s)] = s
		}

		combined := append([]string{}, goSrcs...)
		if cgoEnabled {
			combined = append(combined, cgoSrcs...)
		}
		for i, origSrc := range combined {
			if _, ok := relCoverPath[origSrc]; !ok {
				continue
			}

			var srcName string
			switch coverFormat {
			case "go_cover":
				srcName = origSrc
				if importPath != "" {
					srcName = path.Join(importPath, filepath.Base(origSrc))
				}
			case "lcov":
				// Bazel merges lcov reports across languages and thus assumes
				// that the source file paths are relative to the exec root.
				srcName = relCoverPath[origSrc]
			default:
				return fmt.Errorf("invalid value for -cover_format: %q", coverFormat)
			}

			stem := filepath.Base(origSrc)
			if ext := filepath.Ext(stem); ext != "" {
				stem = stem[:len(stem)-len(ext)]
			}
			coverVar := fmt.Sprintf("Cover_%s_%d_%s", sanitizePathForIdentifier(importPath), i, sanitizePathForIdentifier(stem))
			coverVar = strings.ReplaceAll(coverVar, "_", "Z")
			coverSrc := filepath.Join(workDir, fmt.Sprintf("cover_%d.go", i))
			if err := instrumentForCoverage(goenv, origSrc, srcName, coverVar, coverMode, coverSrc); err != nil {
				return err
			}

			if i < len(goSrcs) {
				goSrcs[i] = coverSrc
				continue
			}

			cgoSrcs[i-len(goSrcs)] = coverSrc
		}
	}

	// If we have cgo, generate separate C and go files, and compile the
	// C files.
	var objFiles []string
	if compilingWithCgo {
		var srcDir string
		if coverMode != "" && cgoGoSrcsForNogoPath != "" {
			// If the package uses Cgo, compile .s and .S files with cgo2, not the Go assembler.
			// Otherwise: the .s/.S files will be compiled with the Go assembler later
			srcDir, goSrcs, objFiles, err = cgo2(goenv, goSrcs, cgoSrcs, cSrcs, cxxSrcs, objcSrcs, objcxxSrcs, sSrcs, hSrcs, packagePath, packageName, cc, cppFlags, cFlags, cxxFlags, objcFlags, objcxxFlags, ldFlags, cgoExportHPath, "")
			if err != nil {
				return err
			}
			// Also run cgo on original source files, not coverage instrumented, if using nogo.
			// The compilation outputs are only used to run cgo, but the generated sources are
			// passed to the separate nogo action via cgoGoSrcsForNogoPath.
			_, _, _, err = cgo2(goenv, goSrcsNogo, cgoSrcsNogo, cSrcs, cxxSrcs, objcSrcs, objcxxSrcs, sSrcs, hSrcs, packagePath, packageName, cc, cppFlags, cFlags, cxxFlags, objcFlags, objcxxFlags, ldFlags, "", cgoGoSrcsForNogoPath)
			if err != nil {
				return err
			}
		} else {
			// If the package uses Cgo, compile .s and .S files with cgo2, not the Go assembler.
			// Otherwise: the .s/.S files will be compiled with the Go assembler later
			srcDir, goSrcs, objFiles, err = cgo2(goenv, goSrcs, cgoSrcs, cSrcs, cxxSrcs, objcSrcs, objcxxSrcs, sSrcs, hSrcs, packagePath, packageName, cc, cppFlags, cFlags, cxxFlags, objcFlags, objcxxFlags, ldFlags, cgoExportHPath, cgoGoSrcsForNogoPath)
			if err != nil {
				return err
			}
		}
		gcFlags = append(gcFlags, createTrimPath(gcFlags, srcDir))
	} else {
		if cgoExportHPath != "" {
			if err := os.WriteFile(cgoExportHPath, nil, 0o666); err != nil {
				return err
			}
		}
		gcFlags = append(gcFlags, createTrimPath(gcFlags, "."))
	}

	importcfgPath, err := checkImportsAndBuildCfg(goenv, importPath, srcs, deps, packageListPath, recompileInternalDeps, compilingWithCgo, coverMode, workDir)
	if err != nil {
		return err
	}

	// Build an embedcfg file mapping embed patterns to filenames.
	// Embed patterns are relative to any one of a list of root directories
	// that may contain embeddable files. Source files containing embed patterns
	// must be in one of these root directories so the pattern appears to be
	// relative to the source file. Due to transitions, source files can reside
	// under Bazel roots different from both those of the go srcs and those of
	// the compilation output. Thus, we have to consider all combinations of
	// Bazel roots embedsrcs and root-relative paths of source files and the
	// output binary.
	var embedRootDirs []string
	for _, root := range embedRoots {
		for _, lookupDir := range embedLookupDirs {
			embedRootDir := abs(filepath.Join(root, lookupDir))
			// Since we are iterating over all combinations of roots and
			// root-relative paths, some resulting paths may not exist and
			// should be filtered out before being passed to buildEmbedcfgFile.
			// Since Bazel uniquified both the roots and the root-relative
			// paths, the combinations are automatically unique.
			if _, err := os.Stat(embedRootDir); err == nil {
				embedRootDirs = append(embedRootDirs, embedRootDir)
			}
		}
	}
	embedcfgPath, err := buildEmbedcfgFile(srcs.goSrcs, embedSrcs, embedRootDirs, workDir)
	if err != nil {
		return err
	}
	if embedcfgPath != "" {
		if !goenv.shouldPreserveWorkDir {
			defer os.Remove(embedcfgPath)
		}
	}

	// If there are Go assembly files and this is go1.12+: generate symbol ABIs.
	// This excludes Cgo packages: they use the C compiler for assembly.
	asmHdrPath := ""
	if len(srcs.sSrcs) > 0 {
		asmHdrPath = filepath.Join(workDir, "go_asm.h")
	}
	var symabisPath string
	if !haveCgo {
		symabisPath, err = buildSymabisFile(goenv, packagePath, srcs.sSrcs, srcs.hSrcs, asmHdrPath)
		if symabisPath != "" {
			if !goenv.shouldPreserveWorkDir {
				defer os.Remove(symabisPath)
			}
		}
		if err != nil {
			return err
		}
	}

	// Compile the filtered .go files.
	if err := compileGo(goenv, goSrcs, packagePath, importcfgPath, embedcfgPath, asmHdrPath, symabisPath, gcFlags, pgoprofile, outLinkObj, outInterfacePath); err != nil {
		return err
	}

	// Compile the .s files with Go's assembler, if this is not a cgo package.
	// Cgo is assembled by cc above.
	if len(srcs.sSrcs) > 0 && !haveCgo {
		includeSet := map[string]struct{}{
			filepath.Join(os.Getenv("GOROOT"), "pkg", "include"): {},
			workDir: {},
		}
		for _, hdr := range srcs.hSrcs {
			includeSet[filepath.Dir(hdr.filename)] = struct{}{}
		}
		includes := make([]string, 0, len(includeSet))
		for inc := range includeSet {
			includes = append(includes, inc)
		}
		sort.Strings(includes)
		for _, inc := range includes {
			asmFlags = append(asmFlags, "-I", inc)
		}
		for i, sSrc := range srcs.sSrcs {
			obj := filepath.Join(workDir, fmt.Sprintf("s%d.o", i))
			if err := asmFile(goenv, sSrc.filename, packagePath, asmFlags, obj); err != nil {
				return err
			}
			objFiles = append(objFiles, obj)
		}
	}

	// Windows resource files (.syso) are treated the same as object files.
	for _, src := range srcs.sysoSrcs {
		objFiles = append(objFiles, src.filename)
	}

	// Pack .o and .syso files into the archive. These may come from cgo generated code,
	// cgo dependencies (cdeps), windows resource file generation, or assembly.
	if len(objFiles) > 0 {
		if err := appendToArchive(goenv, outLinkObj, objFiles); err != nil {
			return err
		}
	}

	return nil
}

func checkImportsAndBuildCfg(goenv *env, importPath string, srcs archiveSrcs, deps []archive, packageListPath string, recompileInternalDeps []string, compilingWithCgo bool, coverMode string, workDir string) (string, error) {
	// Check that the filtered sources don't import anything outside of
	// the standard library and the direct dependencies.
	imports, err := checkImports(srcs.goSrcs, deps, packageListPath, importPath, recompileInternalDeps)
	if err != nil {
		return "", err
	}
	if compilingWithCgo {
		// cgo generated code imports some extra packages.
		imports["runtime/cgo"] = nil
		imports["syscall"] = nil
		imports["unsafe"] = nil
	}
	if coverMode != "" {
		if coverMode == "atomic" {
			imports["sync/atomic"] = nil
		}
		const coverdataPath = "github.com/bazelbuild/rules_go/go/tools/coverdata"
		var coverdata *archive
		for i := range deps {
			if deps[i].importPath == coverdataPath {
				coverdata = &deps[i]
				break
			}
		}
		if coverdata == nil {
			return "", errors.New("coverage requested but coverdata dependency not provided")
		}
		imports[coverdataPath] = coverdata
	}

	// Build an importcfg file for the compiler.
	importcfgPath, err := buildImportcfgFileForCompile(imports, goenv.installSuffix, workDir)
	if err != nil {
		return "", err
	}
	return importcfgPath, nil
}

func compileGo(goenv *env, srcs []string, packagePath, importcfgPath, embedcfgPath, asmHdrPath, symabisPath string, gcFlags []string, pgoprofile, outLinkobjPath, outInterfacePath string) error {
	args := goenv.goTool("compile")
	args = append(args, "-p", packagePath, "-importcfg", importcfgPath, "-pack")
	if embedcfgPath != "" {
		args = append(args, "-embedcfg", embedcfgPath)
	}
	if asmHdrPath != "" {
		args = append(args, "-asmhdr", asmHdrPath)
	}
	if symabisPath != "" {
		args = append(args, "-symabis", symabisPath)
	}
	if pgoprofile != "" {
		args = append(args, "-pgoprofile", pgoprofile)
	}
	args = append(args, gcFlags...)
	args = append(args, "-o", outInterfacePath)
	args = append(args, "-linkobj", outLinkobjPath)
	args = append(args, "--")
	args = append(args, srcs...)
	absArgs(args, []string{"-I", "-o", "-trimpath", "-importcfg"})
	return goenv.runCommand(args)
}

func appendToArchive(goenv *env, outPath string, objFiles []string) error {
	// Use abs to work around long path issues on Windows.
	args := goenv.goTool("pack", "r", abs(outPath))
	args = append(args, objFiles...)
	return goenv.runCommand(args)
}

func createTrimPath(gcFlags []string, path string) string {
	for _, flag := range gcFlags {
		if strings.HasPrefix(flag, "-trimpath=") {
			return flag + ":" + path
		}
	}

	return "-trimpath=" + path
}

func sanitizePathForIdentifier(path string) string {
	return strings.Map(func(r rune) rune {
		if 'A' <= r && r <= 'Z' ||
			'a' <= r && r <= 'z' ||
			'0' <= r && r <= '9' ||
			r == '_' {
			return r
		}
		return '_'
	}, path)
}
