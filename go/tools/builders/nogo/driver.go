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
	"bytes"
	"errors"
	"flag"
	"fmt"
	"regexp"
	"strings"

	"golang.org/x/tools/go/analysis"
)

// These exit codes are the process protocol shared with ../constants.go.
// The bootstrap builder cannot import this package, which depends on x/tools.
const (
	nogoSuccess = iota
	nogoError
	nogoViolation
)

// Run loads a package and runs the supplied analyzers. It returns a diagnostic
// or execution error and the corresponding process exit code. Arguments must
// already have been expanded from Bazel parameter files by the launcher.
func Run(args []string, configs map[string]Config, debug bool, analyzers ...*analysis.Analyzer) (error, int) {
	if err := analysis.Validate(analyzers); err != nil {
		return err, nogoError
	}

	factMap := factMultiFlag{}
	flags := flag.NewFlagSet("nogo", flag.ExitOnError)
	flags.Var(&factMap, "fact", "Import path and file containing facts for that library, separated by '=' (may be repeated)'")
	factsOnly := flags.Bool("facts_only", false, "If true, only fact-producing analyzers are run")
	typesOnly := flags.Bool("types_only", false, "If true, export types without running analyzers")
	importcfg := flags.String("importcfg", "", "The import configuration file")
	goVersion := flags.String("go_version", "", "The SDK Go version from rules_go, without the leading 'go' prefix (for example 1.24.3); nogo normalizes it for go/types")
	packagePath := flags.String("p", "", "The package path (importmap) of the package being compiled")
	xPath := flags.String("x", "", "The file where serialized types and facts should be written")
	nogoFixDir := flags.String("fix_dir", "", "The path of the directory to store the nogo fixes in")
	var ignores []string
	flags.Func("ignore", "Names of files to ignore", func(value string) error {
		ignores = append(ignores, value)
		return nil
	})
	flags.Parse(args)
	srcs := flags.Args()

	packageFile, importMap, err := readImportCfg(*importcfg)
	if err != nil {
		return fmt.Errorf("error parsing importcfg: %v", err), nogoError
	}

	normalizedGoVersion := normalizeGoVersion(*goVersion)

	enabledAnalyzers := analyzers
	if *typesOnly {
		enabledAnalyzers = nil
	}
	diagnostics, pkg, err := checkPackage(configs, enabledAnalyzers, *packagePath, normalizedGoVersion, packageFile, importMap, factMap, *factsOnly, srcs, ignores)
	if err != nil {
		return fmt.Errorf("error running analyzers: %v", err), nogoError
	}

	// Export types and facts even when diagnostics will fail validation. Downstream
	// analyses must not depend on the compiler's private export-data format.
	if *xPath != "" {
		if pkg.illTyped {
			return fmt.Errorf("cannot export ill-typed package: %v", pkg.typeCheckError), nogoError
		}
		if err := writeExport(*xPath, pkg); err != nil {
			return err, nogoError
		}
	}

	fset := pkg.fset

	exitCode := nogoSuccess
	var errMsg bytes.Buffer
	if len(diagnostics) > 0 {
		exitCode = nogoViolation
		if debug {
			// Force actions running nogo to fail to help debug issues.
			exitCode = nogoError
		}
		errMsg.WriteString("errors found by nogo during build-time code analysis:")
		for _, d := range diagnostics {
			fmt.Fprintf(&errMsg, "\n%s: %s (%s)", fset.Position(d.Pos), d.Message, d.analyzerName)
		}
	}

	if errs := saveSuggestedFixes(*nogoFixDir, diagnostics, fset); len(errs) > 0 {
		errMsg.WriteString("\nsaving suggested fixes:")
		for _, err := range errs {
			fmt.Fprintf(&errMsg, "\n%v", err)
		}
	}

	if errMsg.Len() > 0 {
		return errors.New(errMsg.String()), exitCode
	}
	return nil, exitCode
}

// Config determines which source files an analyzer will emit diagnostics for.
// Config values are supplied by the generated launcher.
type Config struct {
	// OnlyFiles is a list of regular expressions that match files an analyzer
	// will emit diagnostics for. When empty, the analyzer will emit diagnostics
	// for all files.
	OnlyFiles []*regexp.Regexp

	// ExcludeFiles is a list of regular expressions that match files that an
	// analyzer will not emit diagnostics for.
	ExcludeFiles []*regexp.Regexp

	// AnalyzerFlags is a map of flag names to flag values which will be passed
	// to Analyzer.Flags. Note that no leading '-' should be present in a flag
	// name
	AnalyzerFlags map[string]string
}

type factMultiFlag map[string]string

func (m *factMultiFlag) String() string {
	if m == nil || len(*m) == 0 {
		return ""
	}
	return fmt.Sprintf("%v", *m)
}

func (m *factMultiFlag) Set(v string) error {
	parts := strings.Split(v, "=")
	if len(parts) != 2 {
		return fmt.Errorf("badly formatted -fact flag: %s", v)
	}
	(*m)[parts[0]] = parts[1]
	return nil
}
