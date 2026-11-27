package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"maps"
	"os"
	"path"
	"path/filepath"
	"reflect"
	"slices"
	"sort"
	"strings"
	"testing"

	"golang.org/x/tools/go/packages"

	"github.com/bazelbuild/rules_go/go/tools/bazel_testing"
)

func TestMain(m *testing.M) {
	bazel_testing.TestMain(m, bazel_testing.Args{
		Main: `
-- BUILD.bazel --
load("@io_bazel_rules_go//go:def.bzl", "go_library", "go_test")

go_library(
    name = "hello",
    srcs = ["hello.go", "hellocgo.go"],
    cgo = True,
    importpath = "example.com/hello",
    visibility = ["//visibility:public"],
)

go_test(
    name = "hello_test",
    srcs = [
        "hello_test.go",
        "hello_external_test.go",
    ],
    embed = [":hello"],
)

go_library(
    name = "incompatible",
    srcs = ["incompatible.go"],
    importpath = "example.com/incompatible",
    target_compatible_with = ["@platforms//:incompatible"],
)

-- hello.go --
package hello

import (
	"os"
	"fmt"
)

func main() {
	fmt.Fprintln(os.Stderr, "Hello World!")
}

-- hellocgo.go --
package hello

/*
int num(void) { return 42; }
*/
import "C"

func run() {
    println(C.num)
}

-- hello_test.go --
package hello

import "testing"

func TestHelloInternal(t *testing.T) {}

-- hello_external_test.go --
package hello_test

import "testing"

func TestHelloExternal(t *testing.T) {}

-- incompatible.go --
//go:build ignore
package hello

-- subhello/BUILD.bazel --
load("@io_bazel_rules_go//go:def.bzl", "go_library", "go_test")

go_library(
    name = "subhello",
    srcs = ["subhello.go"],
    importpath = "example.com/hello/subhello",
    visibility = ["//visibility:public"],
)

-- subhello/subhello.go --
package subhello

import "os"

func main() {
	fmt.Fprintln(os.Stderr, "Subdirectory Hello World!")
}

-- unattached.go --
package unattached

// not mentioned in any target

-- offline/dep/BUILD.bazel --
load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "dep",
    srcs = ["dep.go"],
    importpath = "example.com/offline/dep",
    visibility = ["//visibility:public"],
)

-- offline/dep/dep.go --
package dep

func Value() int {
	return 1
}

-- offline/lib/BUILD.bazel --
load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "lib",
    srcs = ["lib.go"],
    importpath = "example.com/offline/lib",
    visibility = ["//visibility:public"],
    deps = ["//offline/dep"],
)

-- offline/lib/lib.go --
package lib

import (
	"fmt"

	"example.com/offline/dep"
)

func Hello() string {
	return fmt.Sprintf("%d", dep.Value())
}
`,
	})
}

// The repository part depends on whether rules_go is used through WORKSPACE or
// Bzlmod, and under Bzlmod on how the module was brought in, so only the
// package and target are matched.
const osPkgSuffix = "//stdlib:os"

func isOsPkgID(id string) bool {
	return strings.HasSuffix(id, osPkgSuffix)
}

func TestBaseFileLookup(t *testing.T) {
	resp := runForTest(t, packages.DriverRequest{}, ".", "file=hello.go")

	t.Run("roots", func(t *testing.T) {
		if len(resp.Roots) != 1 {
			t.Errorf("Expected 1 package root: %+v", resp.Roots)
			return
		}

		if !strings.HasSuffix(resp.Roots[0], "//:hello") {
			t.Errorf("Unexpected package id: %q", resp.Roots[0])
			return
		}
	})

	t.Run("package", func(t *testing.T) {
		pkg := findPackageByID(resp.Packages, resp.Roots[0])
		if pkg == nil {
			t.Errorf("Expected to find %q in resp.Packages", resp.Roots[0])
			return
		}

		wantCompiledGoFiles := map[string]struct{}{
			"hello.go":         {},
			"_cgo_gotypes.go":  {},
			"_cgo_imports.go":  {},
			"hellocgo.cgo1.go": {},
		}
		for _, file := range pkg.CompiledGoFiles {
			key := filepath.Base(file)
			if _, ok := wantCompiledGoFiles[key]; !ok {
				t.Errorf("Unexpected compiled file: %q", key)
			} else {
				delete(wantCompiledGoFiles, key)
			}
		}
		if len(wantCompiledGoFiles) != 0 {
			t.Errorf("Expected compiled files not found: %+v", slices.Sorted(maps.Keys(wantCompiledGoFiles)))
		}

		wantGoFiles := map[string]struct{}{
			"hello.go":    {},
			"hellocgo.go": {},
		}
		for _, file := range pkg.GoFiles {
			key := filepath.Base(file)
			if _, ok := wantGoFiles[key]; !ok {
				t.Errorf("Unexpected go file: %q", key)
			} else {
				delete(wantGoFiles, key)
			}
		}
		if len(wantGoFiles) != 0 {
			t.Errorf("Expected go files not found: %+v", slices.Sorted(maps.Keys(wantGoFiles)))
		}
		wantImports := []string{"os", "fmt", "runtime/cgo", "syscall", "unsafe"}
		sort.Strings(wantImports)
		gotImports := slices.Sorted(maps.Keys(pkg.Imports))
		if !reflect.DeepEqual(gotImports, wantImports) {
			t.Errorf("Expected imports: %+v got: %+v\n", wantImports, gotImports)
			return
		}

		if !isOsPkgID(pkg.Imports["os"].ID) {
			t.Errorf("Expected os import to map to a package ending in %q:\n%+v", osPkgSuffix, pkg)
			return
		}
	})

	t.Run("dependency", func(t *testing.T) {
		var osPkg *packages.Package
		for _, p := range resp.Packages {
			if isOsPkgID(p.ID) {
				osPkg = p
			}
		}

		if osPkg == nil {
			t.Errorf("Expected os package to be included:\n%+v", osPkg)
			return
		}
	})
}

func TestRelativeFileLookup(t *testing.T) {
	resp := runForTest(t, packages.DriverRequest{}, "subhello", "file=./subhello.go")

	t.Run("roots", func(t *testing.T) {
		if len(resp.Roots) != 1 {
			t.Errorf("Expected 1 package root: %+v", resp.Roots)
			return
		}

		if !strings.HasSuffix(resp.Roots[0], "//subhello:subhello") {
			t.Errorf("Unexpected package id: %q", resp.Roots[0])
			return
		}
	})

	t.Run("package", func(t *testing.T) {
		pkg := findPackageByID(resp.Packages, resp.Roots[0])

		if pkg == nil {
			t.Errorf("Expected to find %q in resp.Packages", resp.Roots[0])
			return
		}

		if len(pkg.CompiledGoFiles) != 1 || len(pkg.GoFiles) != 1 ||
			path.Base(pkg.GoFiles[0]) != "subhello.go" || path.Base(pkg.CompiledGoFiles[0]) != "subhello.go" {
			t.Errorf("Expected to find 1 file (subhello.go) in (Compiled)GoFiles:\n%+v", pkg)
			return
		}
	})
}

func TestRelativePatternWildcardLookup(t *testing.T) {
	resp := runForTest(t, packages.DriverRequest{}, "subhello", "./...")

	t.Run("roots", func(t *testing.T) {
		if len(resp.Roots) != 1 {
			t.Errorf("Expected 1 package root: %+v", resp.Roots)
			return
		}

		if !strings.HasSuffix(resp.Roots[0], "//subhello:subhello") {
			t.Errorf("Unexpected package id: %q", resp.Roots[0])
			return
		}
	})

	t.Run("package", func(t *testing.T) {
		pkg := findPackageByID(resp.Packages, resp.Roots[0])

		if pkg == nil {
			t.Errorf("Expected to find %q in resp.Packages", resp.Roots[0])
			return
		}

		if len(pkg.CompiledGoFiles) != 1 || len(pkg.GoFiles) != 1 ||
			path.Base(pkg.GoFiles[0]) != "subhello.go" || path.Base(pkg.CompiledGoFiles[0]) != "subhello.go" {
			t.Errorf("Expected to find 1 file (subhello.go) in (Compiled)GoFiles:\n%+v", pkg)
			return
		}
	})
}

func TestExternalTests(t *testing.T) {
	resp := runForTest(t, packages.DriverRequest{}, ".", "file=hello_external_test.go")
	if len(resp.Roots) != 2 {
		t.Errorf("Expected exactly two roots for package: %+v", resp.Roots)
	}

	var testId, xTestId string
	for _, id := range resp.Roots {
		if strings.HasSuffix(id, "_xtest") {
			xTestId = id
		} else {
			testId = id
		}
	}

	for _, p := range resp.Packages {
		if p.ID == xTestId {
			if !strings.HasSuffix(p.PkgPath, "_test") {
				t.Errorf("PkgPath missing _test suffix")
			}
			assertSuffixesInList(t, p.GoFiles, "/hello_external_test.go")
		} else if p.ID == testId {
			assertSuffixesInList(t, p.GoFiles, "/hello.go", "/hello_test.go")
		}
	}
}

func TestOverlay(t *testing.T) {
	// format filepaths for overlay request using working directory
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}

	// format filepaths for overlay request
	helloPath := path.Join(wd, "hello.go")
	subhelloPath := path.Join(wd, "subhello/subhello.go")

	expectedImportsPerFile := map[string][]string{
		helloPath:    {"fmt", "runtime/cgo", "syscall", "unsafe"},
		subhelloPath: {"os", "encoding/json"},
	}

	overlayDriverRequest := packages.DriverRequest{
		Overlay: map[string][]byte{
			helloPath: []byte(`
				package hello
				import "fmt"
				import "unknown/unknown-package"
				func main() {
					invalid code

				}`),
			subhelloPath: []byte(`
				package subhello
				import "os"
				import "encoding/json"
				func main() {
					fmt.Fprintln(os.Stderr, "Subdirectory Hello World!")
				}
			`),
		},
	}

	// run the driver with the overlay
	helloResp := runForTest(t, overlayDriverRequest, ".", "file=hello.go")
	subhelloResp := runForTest(t, overlayDriverRequest, "subhello", "file=subhello.go")

	// get root packages
	helloPkg := findPackageByID(helloResp.Packages, helloResp.Roots[0])
	subhelloPkg := findPackageByID(subhelloResp.Packages, subhelloResp.Roots[0])
	if helloPkg == nil {
		t.Fatalf("hello package not found in response root %q", helloResp.Roots[0])
	}
	if subhelloPkg == nil {
		t.Fatalf("subhello package not found in response %q", subhelloResp.Roots[0])
	}

	helloPkgImportPaths := keysFromMap(helloPkg.Imports)
	subhelloPkgImportPaths := keysFromMap(subhelloPkg.Imports)

	expectSetEquality(t, expectedImportsPerFile[helloPath], helloPkgImportPaths, "hello imports")
	expectSetEquality(t, expectedImportsPerFile[subhelloPath], subhelloPkgImportPaths, "subhello imports")
}

// TestIncompatible checks that a target that can be queried but not analyzed
// does not appear in .Roots.
func TestIncompatible(t *testing.T) {
	resp := runForTest(t, packages.DriverRequest{}, ".", "./...")

	rootLabels := make(map[string]bool)
	for _, root := range resp.Roots {
		rootLabels[root] = true
	}

	// Verify //:hello is in .Roots and check whether its label starts with
	// "@@" (bzlmod) or "@" (not bzlmod).
	var incompatibleLabel string
	if rootLabels["@@//:hello"] {
		incompatibleLabel = "@@//:incompatible"
	} else if rootLabels["@//:hello"] {
		incompatibleLabel = "@//:incompatible"
	} else {
		t.Fatalf("response does not contain //:hello; roots were %s", strings.Join(resp.Roots, ", "))
	}

	// Verify //:incompatible is NOT in .Roots.
	if rootLabels[incompatibleLabel] {
		t.Fatalf("response contains root %s", incompatibleLabel)
	}
}

func TestUnattached(t *testing.T) {
	runForTestExpectError(t, "found no labels matching the requests", packages.DriverRequest{}, ".", "file=unattached.go")
}

func TestPkgJSONList(t *testing.T) {
	restoreEnv := scrubTestEnv(t)
	defer restoreEnv()

	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	oldWorkspaceRoot := workspaceRoot
	oldBuildWorkingDirectory := buildWorkingDirectory
	workspaceRoot = wd
	buildWorkingDirectory = wd
	defer func() {
		workspaceRoot = oldWorkspaceRoot
		buildWorkingDirectory = oldBuildWorkingDirectory
	}()

	ctx := context.Background()
	bazel, err := NewBazel(ctx, bazelBin, workspaceRoot, buildWorkingDirectory, bazelCommonFlags, bazelStartupFlags)
	if err != nil {
		t.Fatalf("NewBazel: %v", err)
	}
	builder, err := NewBazelJSONBuilder(bazel, false)
	if err != nil {
		t.Fatalf("NewBazelJSONBuilder: %v", err)
	}

	oldBuildFlags := bazelBuildFlags
	bazelBuildFlags = append(append([]string{}, bazelBuildFlags...), "--@io_bazel_rules_go//go/config:export_stdlib=true")
	defer func() { bazelBuildFlags = oldBuildFlags }()

	mode := packages.NeedName | packages.NeedFiles | packages.NeedCompiledGoFiles |
		packages.NeedImports | packages.NeedExportFile
	jsonFiles, err := builder.Build(ctx, []string{"//offline/lib:lib"}, mode)
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	if len(jsonFiles) == 0 {
		t.Fatal("Build returned no .pkg.json files")
	}

	listFile, err := os.CreateTemp("", "pkg_json_list_*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(listFile.Name())
	for _, f := range jsonFiles {
		if _, err := fmt.Fprintln(listFile, f); err != nil {
			t.Fatal(err)
		}
	}
	if err := listFile.Close(); err != nil {
		t.Fatal(err)
	}

	oldList := pkgJSONList
	pkgJSONList = listFile.Name()
	defer func() { pkgJSONList = oldList }()

	// Any bazel invocation from here on fails the test.
	oldBazelBin := bazelBin
	bazelBin = filepath.Join(t.TempDir(), "no-bazel")
	defer func() { bazelBin = oldBazelBin }()

	execRoot := bazel.ExecutionRoot()
	if err := os.Chdir(execRoot); err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(wd)

	resp := runForTest(t, packages.DriverRequest{Mode: mode}, ".", "example.com/offline/lib")

	if len(resp.Roots) != 1 {
		t.Fatalf("expected 1 root, got %v", resp.Roots)
	}
	root := findPackageByID(resp.Packages, resp.Roots[0])
	if root == nil {
		t.Fatalf("root package missing: %s", resp.Roots[0])
	}
	if root.PkgPath != "example.com/offline/lib" {
		t.Fatalf("PkgPath: got %q", root.PkgPath)
	}
	if len(root.GoFiles) == 0 {
		t.Fatal("root has no GoFiles")
	}
	for _, f := range root.GoFiles {
		if _, err := os.Stat(f); err != nil {
			t.Errorf("GoFile %q: %v", f, err)
		}
	}

	var depPkg, fmtPkg *packages.Package
	for _, p := range resp.Packages {
		switch p.PkgPath {
		case "example.com/offline/dep":
			depPkg = p
		case "fmt":
			fmtPkg = p
		}
	}
	if depPkg == nil {
		t.Fatal("dep package missing")
	}
	if depPkg.ExportFile == "" {
		t.Fatal("dep has no ExportFile")
	}
	if _, err := os.Stat(depPkg.ExportFile); err != nil {
		t.Errorf("dep ExportFile: %v", err)
	}
	if fmtPkg == nil {
		t.Fatal("fmt package missing")
	}
	if fmtPkg.ExportFile == "" {
		t.Fatal("fmt has no ExportFile")
	}
	if _, err := os.Stat(fmtPkg.ExportFile); err != nil {
		t.Errorf("fmt ExportFile: %v", err)
	}

	prefix := runForTest(t, packages.DriverRequest{Mode: mode}, ".", "example.com/offline/...")
	if len(prefix.Roots) != 2 {
		t.Fatalf("example.com/offline/...: expected 2 roots, got %v", prefix.Roots)
	}

	runForTestExpectError(t, "found no packages matching import path", packages.DriverRequest{}, ".", "example.com/missing")
	runForTestExpectError(t, "not supported", packages.DriverRequest{}, ".", "file=lib.go")
}

func runForTest(
	t *testing.T,
	driverRequest packages.DriverRequest,
	relativeWorkingDir string,
	args ...string) packages.DriverResponse {
	t.Helper()
	return runForTestExpectError(t, "", driverRequest, relativeWorkingDir, args...)
}

func runForTestExpectError(
	t *testing.T,
	wantError string,
	driverRequest packages.DriverRequest,
	relativeWorkingDir string,
	args ...string) packages.DriverResponse {
	t.Helper()

	restoreEnv := scrubTestEnv(t)
	defer restoreEnv()

	// Set workspaceRoot and buildWorkingDirectory global variable.
	// It's initialized to the BUILD_WORKSPACE_DIRECTORY environment variable
	// before this point.
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	oldWorkspaceRoot := workspaceRoot
	oldBuildWorkingDirectory := buildWorkingDirectory
	workspaceRoot = wd
	buildWorkingDirectory = filepath.Join(wd, relativeWorkingDir)
	defer func() {
		workspaceRoot = oldWorkspaceRoot
		buildWorkingDirectory = oldBuildWorkingDirectory
	}()

	driverRequestJson, err := json.Marshal(driverRequest)
	if err != nil {
		t.Fatalf("Error serializing driver request: %v\n", err)
	}
	in := bytes.NewReader(driverRequestJson)
	out := &bytes.Buffer{}
	err = run(context.Background(), in, out, args)
	if err == nil && wantError != "" {
		t.Fatal("unexpected success")
	} else if err != nil {
		errMsg := err.Error()
		if wantError == "" {
			t.Fatalf("running gopackagesdriver: %s", errMsg)
		} else if !strings.Contains(errMsg, wantError) {
			t.Fatalf("running gopackagesdriver: %s; error did not contain %q", errMsg, wantError)
		}
		return packages.DriverResponse{}
	}
	var resp packages.DriverResponse
	if err := json.Unmarshal(out.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshaling response: %v", err)
	}
	return resp
}

// scrubTestEnv removes most environment variables, other than those on an
// allowlist, and returns a function that restores them.
//
// Bazel sets TEST_* and RUNFILES_* and a bunch of other variables.
// If Bazel is invoked when these variables, it assumes (correctly)
// that it's being invoked by a test, and it does different things that
// we don't want. For example, it randomizes the output directory, which
// is extremely expensive here. Our test framework creates an output
// directory shared among go_bazel_tests and points to it using .bazelrc.
//
// This only works if TEST_TMPDIR is not set when invoking bazel.
// bazel_testing.BazelCmd normally unsets that, but since gopackagesdriver
// invokes bazel directly, we need to unset it here.
func scrubTestEnv(t *testing.T) func() {
	t.Helper()
	allowEnv := map[string]struct{}{
		"HOME":        {},
		"PATH":        {},
		"PWD":         {},
		"SYSTEMDRIVE": {},
		"SYSTEMROOT":  {},
		"TEMP":        {},
		"TMP":         {},
		"TZ":          {},
		"USER":        {},
	}
	var oldEnv []string
	for _, env := range os.Environ() {
		key, value, cut := strings.Cut(env, "=")
		if !cut {
			continue
		}
		if _, allowed := allowEnv[key]; !allowed && !strings.HasPrefix(key, "GOPACKAGES") {
			os.Unsetenv(key)
			oldEnv = append(oldEnv, key, value)
		}
	}
	return func() {
		for i := 0; i < len(oldEnv); i += 2 {
			os.Setenv(oldEnv[i], oldEnv[i+1])
		}
	}
}

func assertSuffixesInList(t *testing.T, list []string, expectedSuffixes ...string) {
	t.Helper()
	for _, suffix := range expectedSuffixes {
		itemFound := false
		for _, listItem := range list {
			itemFound = itemFound || strings.HasSuffix(listItem, suffix)
		}

		if !itemFound {
			t.Errorf("Expected suffix %q in list, but was not found: %+v", suffix, list)
		}
	}
}

// expectSetEquality checks if two slices are equal sets and logs an error if they are not
func expectSetEquality(t *testing.T, expected []string, actual []string, setName string) {
	t.Helper()
	if !equalSets(expected, actual) {
		t.Errorf("Expected %s %v, got %s %v", setName, expected, actual, setName)
	}
}
