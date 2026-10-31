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
	"encoding/gob"
	"errors"
	"fmt"
	"go/ast"
	"go/token"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"sync"

	"golang.org/x/tools/go/analysis"
)

const nogoBaseConfigName = "_base"

func setAnalyzerFlags(a *analysis.Analyzer, flags map[string]string) error {
	for flagKey, flagVal := range flags {
		if strings.HasPrefix(flagKey, "-") {
			return fmt.Errorf("%s: flag should not begin with '-': %s", a.Name, flagKey)
		}
		if flag := a.Flags.Lookup(flagKey); flag == nil {
			return fmt.Errorf("%s: unrecognized flag: %s", a.Name, flagKey)
		}
		if err := a.Flags.Set(flagKey, flagVal); err != nil {
			return fmt.Errorf("%s: invalid value for flag: %s=%s: %w", a.Name, flagKey, flagVal, err)
		}
	}
	return nil
}

// checkPackage runs all the given analyzers on the specified package and
// returns the source code diagnostics that the must be printed in the build log.
// It returns an empty string if no source code diagnostics need to be printed.
//
// This implementation was adapted from that of golang.org/x/tools/go/checker/internal/checker.
func checkPackage(configs map[string]Config, analyzers []*analysis.Analyzer, packagePath, goVersion string, packageFile, importMap, factMap map[string]string, factsOnly bool, filenames, ignoreFiles []string) ([]diagnosticEntry, *goPackage, error) {
	// Register fact types and establish dependencies between analyzers.
	actions := make(map[*analysis.Analyzer]*action)
	var visit func(a *analysis.Analyzer) *action
	visit = func(a *analysis.Analyzer) *action {
		act, ok := actions[a]
		if !ok {
			act = &action{a: a}
			actions[a] = act
			for _, f := range a.FactTypes {
				act.usesFacts = true
				gob.Register(f)
			}
			act.deps = make([]*action, len(a.Requires))
			for i, req := range a.Requires {
				dep := visit(req)
				if dep.usesFacts {
					act.usesFacts = true
				}
				act.deps[i] = dep
			}
		}
		return act
	}

	// We populate flags for analyzers and their subanalyzers to depth of one. Some analyzers require to provide
	// flags to their dependencies e.g. nilaway has specific nilaway_config subanalyzer.
	for _, a := range analyzers {
		if cfg, ok := configs[a.Name]; ok {
			if err := setAnalyzerFlags(a, cfg.AnalyzerFlags); err != nil {
				return nil, nil, err
			}
		}
		for _, ra := range a.Requires {
			if cfg, ok := configs[ra.Name]; ok {
				if err := setAnalyzerFlags(ra, cfg.AnalyzerFlags); err != nil {
					return nil, nil, err
				}
			}
		}
	}

	// In facts-only mode diagnostics are discarded, so we only need to run
	// analyzers that produce facts.
	//
	// Note that this set may be disjoint from the initial set of analyzers: root
	// analyzers may consume results from required analyzers which themselves use
	// facts.
	if factsOnly {
		analyzers = factProducers(analyzers)
	}
	roots := make([]*action, 0, len(analyzers))
	for _, a := range analyzers {
		roots = append(roots, visit(a))
	}

	// Load the package, including AST, types, and facts.
	imp := newImporter(importMap, packageFile, factMap)
	pkg, err := load(packagePath, goVersion, imp, filenames, len(roots) > 0)
	if err != nil {
		return nil, nil, fmt.Errorf("error loading package: %v", err)
	}

	// Even without analyzers, a well-typed package is required to export types.
	if pkg.illTyped && len(roots) == 0 {
		return nil, nil, pkg.typeCheckError
	}
	for _, act := range actions {
		act.pkg = pkg
	}

	ignoreFilesSet := map[string]struct{}{}
	for _, ignore := range ignoreFiles {
		ignoreFilesSet[ignore] = struct{}{}
	}
	// Process nolint directives similar to golangci-lint.
	// Also skip over fully ignored files.
	for _, f := range pkg.syntax {
		if _, ok := ignoreFilesSet[pkg.fset.Position(f.Pos()).Filename]; ok {
			for _, act := range actions {
				act.nolint = append(act.nolint, &Range{
					from: pkg.fset.Position(f.Pos()),
					to:   pkg.fset.Position(f.End()).Line,
				})
			}
			continue
		}
		// CommentMap will correctly associate comments to the largest node group
		// applicable. This handles inline comments that might trail a large
		// assignment and will apply the comment to the entire assignment.
		commentMap := ast.NewCommentMap(pkg.fset, f, f.Comments)
		for node, groups := range commentMap {
			rng := &Range{
				from: pkg.fset.Position(node.Pos()),
				to:   pkg.fset.Position(node.End()).Line,
			}
			for _, group := range groups {
				for _, comm := range group.List {
					linters, ok := parseNolint(comm.Text)
					if !ok {
						continue
					}
					for analyzer, act := range actions {
						if linters == nil || linters[analyzer.Name] {
							act.nolint = append(act.nolint, rng)
						}
					}
				}
			}
		}
	}

	// Execute the analyzers.
	execAll(roots)

	diagnostics, err := checkAnalysisResults(configs, roots, pkg)
	return diagnostics, pkg, err
}

type Range struct {
	from token.Position
	to   int
}

// factProducers returns the set of analyzers that declare facts among the
// transitive closure of as.
func factProducers(as []*analysis.Analyzer) []*analysis.Analyzer {
	var producers []*analysis.Analyzer
	seen := make(map[*analysis.Analyzer]bool)
	var visit func(a *analysis.Analyzer)
	visit = func(a *analysis.Analyzer) {
		if seen[a] {
			return
		}
		seen[a] = true
		if len(a.FactTypes) > 0 {
			producers = append(producers, a)
		}
		for _, req := range a.Requires {
			visit(req)
		}
	}
	for _, a := range as {
		visit(a)
	}
	return producers
}

// An action represents one unit of analysis work: the application of
// one analysis to one package. Actions form a DAG within a
// package (as different analyzers are applied, either in sequence or
// parallel).
type action struct {
	once        sync.Once
	a           *analysis.Analyzer
	pass        *analysis.Pass
	pkg         *goPackage
	deps        []*action
	inputs      map[*analysis.Analyzer]interface{}
	result      interface{}
	diagnostics []analysis.Diagnostic
	usesFacts   bool
	err         error
	nolint      []*Range
}

func (act *action) String() string {
	return fmt.Sprintf("%s@%s", act.a, act.pkg)
}

func execAll(actions []*action) {
	var wg sync.WaitGroup
	wg.Add(len(actions))
	for _, act := range actions {
		go func(act *action) {
			defer wg.Done()
			act.exec()
		}(act)
	}
	wg.Wait()
}

func (act *action) exec() { act.once.Do(act.execOnce) }

func (act *action) execOnce() {
	// Analyze dependencies.
	execAll(act.deps)

	// Report an error if any dependency failed.
	var failed []string
	for _, dep := range act.deps {
		if dep.err != nil {
			failed = append(failed, dep.String())
		}
	}
	if failed != nil {
		sort.Strings(failed)
		act.err = fmt.Errorf("failed prerequisites: %s", strings.Join(failed, ", "))
		return
	}

	// Plumb the output values of the dependencies
	// into the inputs of this action.
	inputs := make(map[*analysis.Analyzer]interface{})
	for _, dep := range act.deps {
		// Same package, different analysis (horizontal edge):
		// in-memory outputs of prerequisite analyzers
		// become inputs to this analysis pass.
		inputs[dep.a] = dep.result
	}

	ignoreNolintReporter := func(d analysis.Diagnostic) {
		pos := act.pkg.fset.Position(d.Pos)
		for _, rng := range act.nolint {
			// The list of nolint ranges is built for the entire package. Make sure we
			// only apply ranges to the correct file.
			if pos.Filename != rng.from.Filename {
				continue
			}
			if pos.Line < rng.from.Line || pos.Line > rng.to {
				continue
			}
			// Found a nolint range. Ignore the issue.
			return
		}
		act.diagnostics = append(act.diagnostics, d)
	}

	// Run the analysis.
	factFilter := make(map[reflect.Type]bool)
	for _, f := range act.a.FactTypes {
		factFilter[reflect.TypeOf(f)] = true
	}
	pass := &analysis.Pass{
		Analyzer:          act.a,
		Fset:              act.pkg.fset,
		Files:             act.pkg.syntax,
		Pkg:               act.pkg.types,
		TypesInfo:         act.pkg.typesInfo,
		ResultOf:          inputs,
		Report:            ignoreNolintReporter,
		ImportPackageFact: act.pkg.facts.ImportPackageFact,
		ExportPackageFact: act.pkg.facts.ExportPackageFact,
		ImportObjectFact:  act.pkg.facts.ImportObjectFact,
		ExportObjectFact:  act.pkg.facts.ExportObjectFact,
		AllPackageFacts:   func() []analysis.PackageFact { return act.pkg.facts.AllPackageFacts(factFilter) },
		AllObjectFacts:    func() []analysis.ObjectFact { return act.pkg.facts.AllObjectFacts(factFilter) },
		TypesSizes:        act.pkg.typesSizes,
	}
	act.pass = pass

	var err error
	defer func() {
		if r := recover(); r != nil {
			// If the analyzer panics, we catch it here and return an error.
			act.err = fmt.Errorf("panic: %v", r)
		}
	}()
	if !act.pkg.illTyped || pass.Analyzer.RunDespiteErrors {
		act.result, err = pass.Analyzer.Run(pass)
		if err == nil {
			if got, want := reflect.TypeOf(act.result), pass.Analyzer.ResultType; got != want {
				err = fmt.Errorf(
					"internal error: on package %s, analyzer %s returned a result of type %v, but declared ResultType %v",
					pass.Pkg.Path(), pass.Analyzer, got, want)
			}
		}
	}
	act.err = err
}

// checkAnalysisResults checks the analysis diagnostics in the given actions
// and returns a string containing all the diagnostics that should be printed
// to the build log.
func checkAnalysisResults(configs map[string]Config, actions []*action, pkg *goPackage) ([]diagnosticEntry, error) {
	var diagnostics []diagnosticEntry
	var errs []error
	cwd, err := os.Getwd()
	if cwd == "" || err != nil {
		errs = append(errs, fmt.Errorf("nogo failed to get CWD: %w", err))
	}
	numSkipped := 0
	for _, act := range actions {
		if act.pkg.illTyped && !act.a.RunDespiteErrors {
			// Don't report type-checking errors once per analyzer.
			numSkipped++
			continue
		}
		if act.err != nil {
			// Analyzer failed.
			errs = append(errs, fmt.Errorf("analyzer %q failed: %v", act.a.Name, act.err))
			continue
		}
		if len(act.diagnostics) == 0 {
			continue
		}
		var currentConfig Config
		// Use the base config if it exists.
		if baseConfig, ok := configs[nogoBaseConfigName]; ok {
			currentConfig = baseConfig
		}
		// Overwrite the config with the desired config. Any unset fields
		// in the config will default to the base config.
		if actionConfig, ok := configs[act.a.Name]; ok {
			if actionConfig.AnalyzerFlags != nil {
				currentConfig.AnalyzerFlags = actionConfig.AnalyzerFlags
			}
			if actionConfig.OnlyFiles != nil {
				currentConfig.OnlyFiles = actionConfig.OnlyFiles
			}
			if actionConfig.ExcludeFiles != nil {
				currentConfig.ExcludeFiles = actionConfig.ExcludeFiles
			}
		}

		if currentConfig.OnlyFiles == nil && currentConfig.ExcludeFiles == nil {
			for _, diag := range act.diagnostics {
				diagnostics = append(diagnostics, diagnosticEntry{Diagnostic: diag, analyzerName: act.a.Name})
			}
			continue
		}
		// Discard diagnostics based on the analyzer configuration.
		for _, d := range act.diagnostics {
			// NOTE(golang.org/issue/31008): nilness does not set positions,
			// so don't assume the position is valid.
			p := pkg.fset.Position(d.Pos)
			filename := "-"
			if p.IsValid() {
				filename = p.Filename
			}
			if cwd != "" {
				if relname, err := filepath.Rel(cwd, filename); err == nil {
					filename = relname
				}
			}
			include := true
			if len(currentConfig.OnlyFiles) > 0 {
				// This analyzer emits diagnostics for only a set of files.
				include = false
				for _, pattern := range currentConfig.OnlyFiles {
					if pattern.MatchString(filename) {
						include = true
						break
					}
				}
			}
			if include {
				for _, pattern := range currentConfig.ExcludeFiles {
					if pattern.MatchString(filename) {
						include = false
						break
					}
				}
			}
			if include {
				diagnostics = append(diagnostics, diagnosticEntry{Diagnostic: d, analyzerName: act.a.Name})
			}
		}
	}
	if numSkipped > 0 {
		errs = append(errs, fmt.Errorf("%d analyzers skipped due to type-checking error: %v", numSkipped, pkg.typeCheckError))
	}
	sort.Slice(diagnostics, func(i, j int) bool {
		return diagnostics[i].Pos < diagnostics[j].Pos
	})

	if len(errs) == 0 {
		return diagnostics, nil
	}

	errMsg := &bytes.Buffer{}
	sep := ""
	for _, err := range errs {
		errMsg.WriteString(sep)
		sep = "\n"
		errMsg.WriteString(err.Error())
	}
	return diagnostics, errors.New(errMsg.String())
}
