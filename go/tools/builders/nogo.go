package main

import (
	"bytes"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

func nogo(args []string) error {
	// Parse arguments.
	args, _, err := expandParamsFiles(args)
	if err != nil {
		return err
	}

	fs := flag.NewFlagSet("GoNogo", flag.ExitOnError)
	goenv := envFlags(fs)
	var unfilteredSrcs, ignoreSrcs, recompileInternalDeps multiFlag
	var deps, facts archiveMultiFlag
	var importPath, packagePath, nogoPath, packageListPath string
	var testFilter string
	var outFactsPath, outPath string
	var coverMode string
	var factsOnly bool
	fs.Var(&unfilteredSrcs, "src", ".go, .c, .cc, .m, .mm, .s, or .S file to be filtered and checked")
	fs.Var(&ignoreSrcs, "ignore_src", ".go, .c, .cc, .m, .mm, .s, or .S file to be filtered and checked, but with its diagnostics ignored")
	fs.Var(&deps, "arc", "Import path, package path, and file name of a direct dependency, separated by '='")
	fs.Var(&facts, "facts", "Import path, package path, and file name of a direct dependency's nogo facts file, separated by '='")
	fs.BoolVar(&factsOnly, "facts_only", false, "If true, only nogo facts are emitted, no nogo checks are run")
	fs.StringVar(&importPath, "importpath", "", "The import path of the package being compiled. Not passed to the compiler, but may be displayed in debug data.")
	fs.StringVar(&packagePath, "p", "", "The package path (importmap) of the package being compiled")
	fs.StringVar(&packageListPath, "package_list", "", "The file containing the list of standard library packages")
	fs.Var(&recompileInternalDeps, "recompile_internal_deps", "The import path of the direct dependencies that needs to be recompiled.")
	fs.StringVar(&coverMode, "cover_mode", "", "The coverage mode to use. Empty if coverage instrumentation should not be added.")
	fs.StringVar(&testFilter, "testfilter", "off", "Controls test package filtering")
	fs.StringVar(&nogoPath, "nogo", "", "The nogo binary")
	fs.StringVar(&outFactsPath, "out_facts", "", "The file to emit serialized nogo facts to")
	fs.StringVar(&outPath, "out", "", "The path of the directory to emit logs and fixes into")

	if err := fs.Parse(args); err != nil {
		return err
	}
	if err := goenv.checkFlagsAndSetGoroot(); err != nil {
		return err
	}
	if importPath == "" {
		importPath = packagePath
	}

	// Filter sources.
	srcs, err := filterAndSplitFiles(append(unfilteredSrcs, ignoreSrcs...))
	if err != nil {
		return err
	}

	err = applyTestFilter(testFilter, &srcs)
	if err != nil {
		return err
	}

	var goSrcs []string
	haveCgo := false
	for _, src := range srcs.goSrcs {
		if src.isCgo {
			haveCgo = true
		} else {
			goSrcs = append(goSrcs, src.filename)
		}
	}

	workDir, cleanup, err := goenv.workDir()
	if err != nil {
		return err
	}
	defer cleanup()

	compilingWithCgo := os.Getenv("CGO_ENABLED") == "1" && haveCgo
	importcfgPath, err := checkImportsAndBuildCfg(goenv, importPath, srcs, deps, packageListPath, recompileInternalDeps, compilingWithCgo, coverMode, workDir)
	if err != nil {
		return err
	}

	return runNogo(workDir, nogoPath, goSrcs, ignoreSrcs, facts, factsOnly, importPath, importcfgPath, outFactsPath, outPath)
}

func runNogo(workDir string, nogoPath string, srcs, ignores []string, facts []archive, factsOnly bool, packagePath, importcfgPath, outFactsPath, outDirPath string) error {
	if len(srcs) == 0 {
		// emit_compilepkg expects a nogo facts file, even if it's empty.
		err := os.WriteFile(outFactsPath, nil, 0o666)
		if err != nil {
			return fmt.Errorf("error writing empty nogo facts file: %v", err)
		}
		return nil
	}

	args := []string{nogoPath}
	args = append(args, "-p", packagePath)
	args = append(args, "-fix_dir", outDirPath)
	args = append(args, "-importcfg", importcfgPath)
	for _, fact := range facts {
		args = append(args, "-fact", fmt.Sprintf("%s=%s", fact.importPath, fact.file))
	}
	if factsOnly {
		args = append(args, "-facts_only")
	}
	args = append(args, "-x", outFactsPath)
	for _, ignore := range ignores {
		args = append(args, "-ignore", ignore)
	}
	args = append(args, srcs...)

	paramsFile := filepath.Join(workDir, "nogo.param")
	if err := writeParamsFile(paramsFile, args[1:]); err != nil {
		return fmt.Errorf("error writing nogo params file: %v", err)
	}

	cmd := exec.Command(args[0], "-param="+paramsFile)
	out := &bytes.Buffer{}
	cmd.Stdout, cmd.Stderr = out, out
	err := cmd.Run()
	if err == nil {
		return nil
	}
	if exitErr, ok := err.(*exec.ExitError); ok {
		if !exitErr.Exited() {
			cmdLine := strings.Join(args, " ")
			return fmt.Errorf("nogo command '%s' exited unexpectedly: %s", cmdLine, exitErr.String())
		}
		prettyOut := relativizePaths(out.Bytes())
		if exitErr.ExitCode() != nogoViolation {
			return errors.New(string(prettyOut))
		}
		outLog, err := os.Create(filepath.Join(outDirPath, nogoLogBasename))
		if err != nil {
			return fmt.Errorf("error creating nogo log file: %v", err)
		}
		defer outLog.Close()
		_, err = outLog.Write(prettyOut)
		if err != nil {
			return fmt.Errorf("error writing nogo log file: %v", err)
		}
		// Do not fail the action if nogo has findings so that facts are
		// still available for downstream targets.
		return nil
	}
	return err
}
