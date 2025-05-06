/* Copyright 2016 The Bazel Authors. All rights reserved.

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

// Go testing support for Bazel.
//
// A Go test comprises three packages:
//
// 1. An internal test package, compiled from the sources of the library being
//    tested and any _test.go files with the same package name.
// 2. An external test package, compiled from _test.go files with a package
//    name ending with "_test".
// 3. A generated main package that imports both packages and initializes the
//    test framework with a list of tests, benchmarks, examples, and fuzz
//    targets read from source files.
//
// This action generates the source code for (3). The equivalent code for
// 'go test' is in $GOROOT/src/cmd/go/internal/load/test.go.

package main

import (
	"flag"
	"fmt"
	"go/ast"
	"go/build"
	"go/doc"
	"go/parser"
	"go/token"
	"os"
	"sort"
	"strings"
	"text/template"
)

type Import struct {
	Name string
	Path string
}

type TestCase struct {
	Package string
	Name    string
}

type Example struct {
	Package   string
	Name      string
	Output    string
	Unordered bool
}

// Cases holds template data.
type Cases struct {
	Imports     []*Import
	Tests       []TestCase
	Benchmarks  []TestCase
	FuzzTargets []TestCase
	Examples    []Example
	TestMain    string
	CoverMode   string
	CoverFormat string
	Pkgname     string
}

// Version returns whether v is a supported Go version (like "go1.18").
func (c *Cases) Version(v string) bool {
	for _, r := range build.Default.ReleaseTags {
		if v == r {
			return true
		}
	}
	return false
}

const testMainTpl = `
package main

// bzltestutil may change the current directory in its init function to emulate
// 'go test' behavior. It must be initialized before user packages.
// In Go 1.20 and earlier, this import declaration must appear before
// imports of user packages. See comment in bzltestutil/init.go.
import "github.com/bazelbuild/rules_go/go/tools/bzltestutil"

import (
	"flag"
	"log"
	"os"
	"os/exec"
{{if .TestMain}}
	"reflect"
{{end}}
	"strconv"
	"strings"
	"testing"
	"testing/internal/testdeps"

{{if ne .CoverMode ""}}
	"github.com/bazelbuild/rules_go/go/tools/coverdata"
{{end}}

{{range $p := .Imports}}
	{{$p.Name}} "{{$p.Path}}"
{{end}}
)

var allTests = []testing.InternalTest{
{{range .Tests}}
	{"{{.Name}}", {{.Package}}.{{.Name}} },
{{end}}
}

var benchmarks = []testing.InternalBenchmark{
{{range .Benchmarks}}
	{"{{.Name}}", {{.Package}}.{{.Name}} },
{{end}}
}

{{if .Version "go1.18"}}
var fuzzTargets = []testing.InternalFuzzTarget{
{{range .FuzzTargets}}
  {"{{.Name}}", {{.Package}}.{{.Name}} },
{{end}}
}
{{end}}

var examples = []testing.InternalExample{
{{range .Examples}}
	{Name: "{{.Name}}", F: {{.Package}}.{{.Name}}, Output: {{printf "%q" .Output}}, Unordered: {{.Unordered}} },
{{end}}
}

func testsInShard() []testing.InternalTest {
	totalShards, err := strconv.Atoi(os.Getenv("TEST_TOTAL_SHARDS"))
	if err != nil || totalShards <= 1 {
		return allTests
	}
	file, err := os.Create(os.Getenv("TEST_SHARD_STATUS_FILE"))
	if err != nil {
		log.Fatalf("Failed to touch TEST_SHARD_STATUS_FILE: %v", err)
	}
	_ = file.Close()
	shardIndex, err := strconv.Atoi(os.Getenv("TEST_SHARD_INDEX"))
	if err != nil || shardIndex < 0 {
		return allTests
	}
	tests := []testing.InternalTest{}
	for i, t := range allTests {
		if i % totalShards == shardIndex {
			tests = append(tests, t)
		}
	}
	return tests
}

func main() {
	// When the test process is originally spawned by Bazel,
	// it should run in a test directory.
	// However, if the test process is re-spawned by the test,
	// it should run wherever that process is set to run.
	//
	// Clearing this environment variable will opt the child process
	// out of the Chdir behavior.
	_ = os.Unsetenv("GO_TEST_RUN_FROM_BAZEL")

	if bzltestutil.ShouldWrap() {
		err := bzltestutil.Wrap("{{.Pkgname}}")
		exitCode := 0
		if xerr, ok := err.(*exec.ExitError); ok {
			exitCode = xerr.ExitCode()
		} else if err != nil {
			log.Print(err)
			exitCode = bzltestutil.TestWrapperAbnormalExit
		}
		os.Exit(exitCode)
	}

	testDeps :=
  {{if eq .CoverFormat "lcov"}}
		bzltestutil.LcovTestDeps{TestDeps: testdeps.TestDeps{}}
  {{else}}
		testdeps.TestDeps{}
  {{end}}
  {{if .Version "go1.18"}}
	m := testing.MainStart(testDeps, testsInShard(), benchmarks, fuzzTargets, examples)
  {{else}}
	m := testing.MainStart(testDeps, testsInShard(), benchmarks, examples)
  {{end}}

	if filter := os.Getenv("TESTBRIDGE_TEST_ONLY"); filter != "" {
		filters := strings.Split(filter, ",")
		var runTests []string
		var skipTests []string

		for _, f := range filters {
			if strings.HasPrefix(f, "-") {
				skipTests = append(skipTests, f[1:])
			} else {
				runTests = append(runTests, f)
			}
		}
		if len(runTests) > 0 {
			flag.Lookup("test.run").Value.Set(strings.Join(runTests, "|"))
		}
		if len(skipTests) > 0 {
			flag.Lookup("test.skip").Value.Set(strings.Join(skipTests, "|"))
		}
	}

	if failfast := os.Getenv("TESTBRIDGE_TEST_RUNNER_FAIL_FAST"); failfast != "" {
		flag.Lookup("test.failfast").Value.Set("true")
	}
{{if eq .CoverFormat "lcov"}}
	panicOnExit0Flag := flag.Lookup("test.paniconexit0").Value
	testDeps.OriginalPanicOnExit = panicOnExit0Flag.(flag.Getter).Get().(bool)
	// Setting this flag provides a way to run hooks right before testing.M.Run() returns.
	panicOnExit0Flag.Set("true")
{{end}}
{{if ne .CoverMode ""}}
	if len(coverdata.Counters) > 0 {
		testing.RegisterCover(testing.Cover{
			Mode: "{{ .CoverMode }}",
			Counters: coverdata.Counters,
			Blocks: coverdata.Blocks,
		})

		if coverageDat, ok := os.LookupEnv("COVERAGE_OUTPUT_FILE"); ok {
			{{if eq .CoverFormat "lcov"}}
			flag.Lookup("test.coverprofile").Value.Set(coverageDat+".cover")
			{{else}}
			flag.Lookup("test.coverprofile").Value.Set(coverageDat)
			{{end}}
		}
	}
	{{end}}

	testTimeout := os.Getenv("TEST_TIMEOUT")
	if testTimeout != "" {
		flag.Lookup("test.timeout").Value.Set(testTimeout+"s")
		bzltestutil.RegisterTimeoutHandler()
	}

	{{if not .TestMain}}
	res := m.Run()
	{{else}}
	{{.TestMain}}(m)
	{{/* See golang.org/issue/34129 and golang.org/cl/219639 */}}
	res := int(reflect.ValueOf(m).Elem().FieldByName("exitCode").Int())
	{{end}}
	os.Exit(res)
}
`

func genTestMain(args []string) error {
	// Prepare our flags
	args, _, err := expandParamsFiles(args)
	if err != nil {
		return err
	}
	imports := multiFlag{}
	sources := multiFlag{}
	flags := flag.NewFlagSet("GoTestGenTest", flag.ExitOnError)
	goenv := envFlags(flags)
	out := flags.String("output", "", "output file to write. Defaults to stdout.")
	coverMode := flags.String("cover_mode", "", "the coverage mode to use")
	coverFormat := flags.String("cover_format", "", "the coverage report type to generate (go_cover or lcov)")
	pkgname := flags.String("pkgname", "", "package name of test")
	flags.Var(&imports, "import", "Packages to import")
	flags.Var(&sources, "src", "Sources to process for tests")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if err := goenv.checkFlagsAndSetGoroot(); err != nil {
		return err
	}
	// Process import args
	importMap := map[string]*Import{}
	for _, imp := range imports {
		parts := strings.Split(imp, "=")
		if len(parts) != 2 {
			return fmt.Errorf("Invalid import %q specified", imp)
		}
		i := &Import{Name: parts[0], Path: parts[1]}
		importMap[i.Name] = i
	}
	// Process source args
	sourceList := []string{}
	sourceMap := map[string]string{}
	for _, s := range sources {
		parts := strings.Split(s, "=")
		if len(parts) != 2 {
			return fmt.Errorf("Invalid source %q specified", s)
		}
		sourceList = append(sourceList, parts[1])
		sourceMap[parts[1]] = parts[0]
	}

	// filter our input file list
	filteredSrcs, err := filterAndSplitFiles(sourceList)
	if err != nil {
		return err
	}
	goSrcs := filteredSrcs.goSrcs

	outFile := os.Stdout
	if *out != "" {
		var err error
		outFile, err = os.Create(*out)
		if err != nil {
			return fmt.Errorf("os.Create(%q): %v", *out, err)
		}
		defer outFile.Close()
	}

	cases := Cases{
		CoverFormat: *coverFormat,
		CoverMode:   *coverMode,
		Pkgname:     *pkgname,
	}

	testFileSet := token.NewFileSet()
	pkgs := map[string]bool{}
	for _, f := range goSrcs {
		parse, err := parser.ParseFile(testFileSet, f.filename, nil, parser.ParseComments)
		if err != nil {
			return fmt.Errorf("ParseFile(%q): %v", f.filename, err)
		}
		pkg := sourceMap[f.filename]
		if strings.HasSuffix(parse.Name.String(), "_test") {
			pkg += "_test"
		}
		for _, e := range doc.Examples(parse) {
			if e.Output == "" && !e.EmptyOutput {
				continue
			}
			cases.Examples = append(cases.Examples, Example{
				Name:      "Example" + e.Name,
				Package:   pkg,
				Output:    e.Output,
				Unordered: e.Unordered,
			})
			pkgs[pkg] = true
		}
		for _, d := range parse.Decls {
			fn, ok := d.(*ast.FuncDecl)
			if !ok {
				continue
			}
			if fn.Recv != nil {
				continue
			}
			if fn.Name.Name == "TestMain" {
				// TestMain is not, itself, a test
				pkgs[pkg] = true
				cases.TestMain = fmt.Sprintf("%s.%s", pkg, fn.Name.Name)
				continue
			}

			// Here we check the signature of the Test* function. To
			// be considered a test:

			// 1. The function should have a single argument.
			if len(fn.Type.Params.List) != 1 {
				continue
			}

			// 2. The function should return nothing.
			if fn.Type.Results != nil {
				continue
			}

			// 3. The only parameter should have a type identified as
			//    *<something>.T
			starExpr, ok := fn.Type.Params.List[0].Type.(*ast.StarExpr)
			if !ok {
				continue
			}
			selExpr, ok := starExpr.X.(*ast.SelectorExpr)
			if !ok {
				continue
			}

			// We do not descriminate on the referenced type of the
			// parameter being *testing.T. Instead we assert that it
			// should be *<something>.T. This is because the import
			// could have been aliased as a different identifier.

			if strings.HasPrefix(fn.Name.Name, "Test") {
				if selExpr.Sel.Name != "T" {
					continue
				}
				pkgs[pkg] = true
				cases.Tests = append(cases.Tests, TestCase{
					Package: pkg,
					Name:    fn.Name.Name,
				})
			}
			if strings.HasPrefix(fn.Name.Name, "Benchmark") {
				if selExpr.Sel.Name != "B" {
					continue
				}
				pkgs[pkg] = true
				cases.Benchmarks = append(cases.Benchmarks, TestCase{
					Package: pkg,
					Name:    fn.Name.Name,
				})
			}
			if strings.HasPrefix(fn.Name.Name, "Fuzz") {
				if selExpr.Sel.Name != "F" {
					continue
				}
				pkgs[pkg] = true
				cases.FuzzTargets = append(cases.FuzzTargets, TestCase{
					Package: pkg,
					Name:    fn.Name.Name,
				})
			}
		}
	}

	for name := range importMap {
		// Set the names for all unused imports to "_"
		if !pkgs[name] {
			importMap[name].Name = "_"
		}
		cases.Imports = append(cases.Imports, importMap[name])
	}
	sort.Slice(cases.Imports, func(i, j int) bool {
		return cases.Imports[i].Name < cases.Imports[j].Name
	})
	tpl := template.Must(template.New("source").Parse(testMainTpl))
	if err := tpl.Execute(outFile, &cases); err != nil {
		return fmt.Errorf("template.Execute(%v): %v", cases, err)
	}
	return nil
}
