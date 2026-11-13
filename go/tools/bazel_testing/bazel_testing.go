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

// Package bazel_testing provides an integration testing framework for
// testing rules_go with Bazel.
//
// Tests may be written by declaring a go_bazel_test target instead of
// a go_test (go_bazel_test is defined in def.bzl here), then calling
// TestMain. Tests are run in a synthetic test workspace. Tests may run
// bazel commands with RunBazel.
package bazel_testing

import (
	"bytes"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"testing"
	"text/template"

	"github.com/bazelbuild/rules_go/go/runfiles"
	"github.com/bazelbuild/rules_go/go/tools/bazel"
	"github.com/bazelbuild/rules_go/go/tools/internal/txtar"
)

// Set via x_defs.
var goRootFile string

var (
	bazelBinary      = flag.String("bazel_binary", "", "Runfile path of the Bazel executable to use")
	bazelPath        = "bazel"
	bazelRunfilesEnv []string
	isolatedRoot     string
)

const (
	// Standard Bazel exit codes.
	// A subset of codes in https://cs.opensource.google/bazel/bazel/+/master:src/main/java/com/google/devtools/build/lib/util/ExitCode.java.
	SUCCESS                    = 0
	BUILD_FAILURE              = 1
	COMMAND_LINE_ERROR         = 2
	TESTS_FAILED               = 3
	NO_TESTS_FOUND             = 4
	RUN_FAILURE                = 6
	ANALYSIS_FAILURE           = 7
	INTERRUPTED                = 8
	LOCK_HELD_NOBLOCK_FOR_LOCK = 9
)

// Args is a list of arguments to TestMain. It's defined as a struct so
// that new optional arguments may be added without breaking compatibility.
type Args struct {
	// Main is a text archive containing files in the main module.
	// The text archive format is parsed by
	// //go/tools/internal/txtar:go_default_library, which is copied from
	// cmd/go/internal/txtar. If this archive does not contain a MODULE.bazel
	// file, a default file will be synthesized.
	Main string

	// Nogo is the nogo target to register with the go_sdk extension. By
	// default, nogo is not used.
	Nogo string

	// NogoIncludes is the list of targets to include for Nogo linting.
	NogoIncludes []string

	// NogoExcludes is the list of targets to include for Nogo linting.
	NogoExcludes []string

	// ModuleFileSuffix is a string that should be appended to the end of the
	// default generated MODULE.bazel file.
	ModuleFileSuffix string

	// SkipHostGoSDK disables wrapping the Go SDK of the enclosing build as
	// @go_sdk. Tests that declare an SDK under that name themselves have to
	// set it.
	SkipHostGoSDK bool

	// SetUp is a function that is executed inside the context of the testing
	// workspace. It is executed once and only once before the beginning of
	// all tests. If SetUp returns a non-nil error, execution is halted and
	// tests cases are not executed.
	SetUp func() error
}

// debug may be set to make the test print the test workspace path and stop
// instead of running tests.
const debug = false

// outputUserRoot is set to the directory where Bazel should put its internal files.
// Since Bazel 2.0.0, this needs to be set explicitly to avoid it defaulting to a
// deeply nested directory within the test, which runs into Windows path length limits.
// Each test uses a private root beneath TEST_TMPDIR.
var outputUserRoot string

// TestMain should be called by tests using this framework from a function named
// "TestMain". For example:
//
//	func TestMain(m *testing.M) {
//	  os.Exit(bazel_testing.TestMain(m, bazel_testing.Args{...}))
//	}
//
// TestMain constructs a set of workspaces and changes the working directory to
// the main workspace.
func TestMain(m *testing.M, args Args) {
	// Defer os.Exit with the correct code. This ensures other deferred cleanup
	// functions are run first.
	code := 1
	defer func() {
		if r := recover(); r != nil {
			fmt.Fprintf(os.Stderr, "panic: %v\n", r)
			code = 1
		}
		os.Exit(code)
	}()

	files, err := bazel.SpliceDelimitedOSArgs("-begin_files", "-end_files")
	if err != nil {
		fmt.Fprint(os.Stderr, err)
		return
	}

	flag.Parse()
	if *bazelBinary != "" {
		bazelPath, err = runfiles.Rlocation(*bazelBinary)
		if err == nil {
			bazelPath, err = filepath.Abs(bazelPath)
		}
		if err == nil {
			bazelRunfilesEnv, err = runfiles.Env()
		}
		if err != nil {
			fmt.Fprintf(os.Stderr, "locating bazel_binary: %v\n", err)
			return
		}
	}

	workspaceDir, cleanup, err := setupWorkspace(args, files)
	defer func() {
		if err := cleanup(); err != nil {
			fmt.Fprintf(os.Stderr, "cleanup warning: %v\n", err)
			// Don't fail the test on a cleanup error.
			// Some operating systems (windows, maybe also darwin) can't reliably
			// delete executable files after they're run.
		}
	}()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return
	}

	if debug {
		fmt.Fprintf(os.Stderr, "test setup in %s\n", workspaceDir)
		interrupted := make(chan os.Signal, 1)
		signal.Notify(interrupted, os.Interrupt)
		<-interrupted
		return
	}

	originalDir, err := os.Getwd()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return
	}
	if err := os.Chdir(workspaceDir); err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		return
	}
	defer os.Chdir(originalDir)
	shutdown := BazelCmd("shutdown")
	shutdown.Dir = workspaceDir
	defer shutdown.Run()

	if args.SetUp != nil {
		if err := args.SetUp(); err != nil {
			fmt.Fprintf(os.Stderr, "test provided SetUp method returned error: %v\n", err)
			return
		}
	}

	code = m.Run()
}

// BazelCmd prepares a nested Bazel command using the selected executable and
// private writable state for this test.
func BazelCmd(args ...string) *exec.Cmd {
	cmd := bazelCommand()
	// Keep system rc support: CI machines may need settings such as IPv6 JVM
	// flags. Ignore home rc, but retain the fixture's workspace rc.
	// See https://github.com/bazelbuild/rules_go/pull/3969#issuecomment-2220405416
	// Set the root on the command line so a fixture replacing .bazelrc cannot
	// discard isolation. Distinct workspaces still get distinct output bases.
	cmd.Args = append(cmd.Args, "--nohome_rc", "--output_user_root="+outputUserRoot)
	cmd.Args = append(cmd.Args, args...)
	return cmd
}

// bazelCommand also serves --version, which cannot accept startup options.
func bazelCommand(args ...string) *exec.Cmd {
	cmd := exec.Command(bazelPath, args...)
	for _, e := range os.Environ() {
		// Filter environment variables set by the bazel test wrapper script.
		// These confuse recursive invocations of Bazel.
		if strings.HasPrefix(e, "TEST_") || strings.HasPrefix(e, "RUNFILES_") {
			continue
		}
		key, _, _ := strings.Cut(e, "=")
		if _, replace := isolatedEnvironment[key]; replace {
			continue
		}
		cmd.Env = append(cmd.Env, e)
	}
	// Declared launchers may need runfiles of their own. Their dependencies
	// are already in the enclosing test's runfiles tree.
	cmd.Env = append(cmd.Env, bazelRunfilesEnv...)
	for key, dir := range isolatedEnvironment {
		cmd.Env = append(cmd.Env, key+"="+filepath.Join(isolatedRoot, dir))
	}
	return cmd
}

var isolatedEnvironment = map[string]string{
	"HOME":           "h",
	"USERPROFILE":    "h",
	"XDG_CACHE_HOME": "c",
	"TMPDIR":         "t",
	"TMP":            "t",
	"TEMP":           "t",
	"TEST_TMPDIR":    "t",
	"GOCACHE":        "c/go-build",
	"GOPATH":         "g",
	"GOMODCACHE":     "g/pkg/mod",
	"BAZELISK_HOME":  "c/bazelisk",
}

// RunBazel invokes a bazel command with a list of arguments.
//
// If the command starts but exits with a non-zero status, a *StderrExitError
// will be returned which wraps the original *exec.ExitError.
func RunBazel(args ...string) error {
	cmd := BazelCmd(args...)

	buf := &bytes.Buffer{}
	cmd.Stderr = io.MultiWriter(os.Stderr, buf)
	err := cmd.Run()
	if eErr, ok := err.(*exec.ExitError); ok {
		eErr.Stderr = buf.Bytes()
		err = &StderrExitError{Err: eErr}
	}
	return err
}

// BazelOutput invokes a bazel command with a list of arguments and returns
// the content of stdout.
//
// If the command starts but exits with a non-zero status, a *StderrExitError
// will be returned which wraps the original *exec.ExitError.
func BazelOutput(args ...string) ([]byte, error) {
	stdout, _, err := BazelOutputWithInput(nil, args...)
	return stdout, err
}

// BazelOutputWithInput invokes a bazel command with a list of arguments and
// an input stream and returns the content of stdout.
//
// If the command starts but exits with a non-zero status, a *StderrExitError
// will be returned which wraps the original *exec.ExitError.
func BazelOutputWithInput(stdin io.Reader, args ...string) ([]byte, []byte, error) {
	cmd := BazelCmd(args...)
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	cmd.Stdout = stdout
	cmd.Stderr = io.MultiWriter(os.Stderr, stderr)
	if stdin != nil {
		cmd.Stdin = stdin
	}
	err := cmd.Run()
	if eErr, ok := err.(*exec.ExitError); ok {
		eErr.Stderr = stderr.Bytes()
		err = &StderrExitError{Err: eErr}
	}
	return stdout.Bytes(), stderr.Bytes(), err
}

// StderrExitError wraps *exec.ExitError and prints the complete stderr output
// from a command.
type StderrExitError struct {
	Err *exec.ExitError
}

func (e *StderrExitError) Error() string {
	sb := &strings.Builder{}
	sb.Write(e.Err.Stderr)
	sb.WriteString(e.Err.Error())
	return sb.String()
}

func (e *StderrExitError) Unwrap() error {
	return e.Err
}

func setupWorkspace(args Args, files []string) (dir string, cleanup func() error, err error) {
	var cleanups []func() error
	cleanup = func() error {
		var firstErr error
		for i := len(cleanups) - 1; i >= 0; i-- {
			if err := cleanups[i](); err != nil && firstErr == nil {
				firstErr = err
			}
		}
		return firstErr
	}
	defer func() {
		if err != nil {
			cleanup()
			cleanup = func() error { return nil }
		}
	}()

	// Own a unique root below TEST_TMPDIR. Never delete or escape another
	// test's directories. Direct callers without TEST_TMPDIR use the OS temp dir.
	cacheDir, err := os.MkdirTemp(os.Getenv("TEST_TMPDIR"), "bt-")
	if err != nil {
		return "", cleanup, err
	}
	cleanups = append(cleanups, func() error {
		if err := os.RemoveAll(cacheDir); !os.IsPermission(err) {
			return err
		}
		// Nested Bazel outputs may contain read-only directories. Repair only
		// directories in our private tree; WalkDir does not follow symlinks to
		// runfiles or SDK inputs owned by the enclosing build.
		if err := filepath.WalkDir(cacheDir, func(path string, d os.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if !d.IsDir() {
				return nil
			}
			info, err := d.Info()
			if err != nil {
				return err
			}
			return os.Chmod(path, info.Mode()|0700)
		}); err != nil {
			return err
		}
		return os.RemoveAll(cacheDir)
	})
	cacheDir, err = filepath.Abs(cacheDir)
	if err != nil {
		return "", cleanup, err
	}
	isolatedRoot = cacheDir
	outputUserRoot = filepath.Join(cacheDir, "o")
	for _, dir := range isolatedEnvironment {
		if err := os.MkdirAll(filepath.Join(cacheDir, dir), 0777); err != nil {
			return "", cleanup, err
		}
	}
	execDir := filepath.Join(cacheDir, "w")

	// Copy or link the files for the tested repository.
	testedRepoDir := filepath.Join(execDir, "tested_repo")
	var singleRepoName string
	for _, f := range files {
		repoName, _, _ := strings.Cut(f, "/")
		if singleRepoName == "" {
			singleRepoName = repoName
		} else if repoName != singleRepoName {
			return "", cleanup, fmt.Errorf("data files from more than one repo are unsupported, got %s and %s", singleRepoName, repoName)
		}
		srcPath, err := runfiles.Rlocation(f)
		if err != nil {
			return "", cleanup, fmt.Errorf("unknown runfile %s: %v", f, err)
		}
		dstPath := filepath.Join(testedRepoDir, strings.TrimPrefix(f, singleRepoName+"/"))
		if err := copyOrLink(dstPath, srcPath); err != nil {
			return "", cleanup, fmt.Errorf("copying %s to %s: %v", srcPath, dstPath, err)
		}
	}
	testedRepoModulePath := filepath.Join(testedRepoDir, "MODULE.bazel")
	var testedModuleName string
	if _, err := os.Stat(testedRepoModulePath); err == nil {
		testedModuleName, err = loadName(testedRepoModulePath)
		if err != nil {
			return "", cleanup, fmt.Errorf("loading module name: %v", err)
		}
	}
	testedRepoWorkspacePath := filepath.Join(testedRepoDir, "WORKSPACE")
	testedModuleRepoName := testedModuleName
	if _, err = os.Stat(testedRepoWorkspacePath); err == nil {
		testedModuleRepoName, err = loadName(testedRepoWorkspacePath)
		if err != nil {
			return "", cleanup, fmt.Errorf("loading workspace name: %v", err)
		}
	}

	// Create the workspace directory.
	mainDir := filepath.Join(execDir, "main")
	if err := os.MkdirAll(mainDir, 0777); err != nil {
		return "", cleanup, err
	}

	// Create a .bazelrc file with the contents of GO_BAZEL_TEST_BAZELFLAGS is set.
	// The test can override this with its own .bazelrc or with flags in commands.
	bazelrcPath := filepath.Join(mainDir, ".bazelrc")
	bazelrcBuf := &bytes.Buffer{}
	fmt.Fprintf(bazelrcBuf, "common --enable_bzlmod\n")
	if outputUserRoot != "" {
		fmt.Fprintf(bazelrcBuf, "startup --output_user_root=%s\n", strconv.Quote(filepath.ToSlash(outputUserRoot)))
	}
	if flags := os.Getenv("GO_BAZEL_TEST_BAZELFLAGS"); flags != "" {
		fmt.Fprintf(bazelrcBuf, "common %s\n", flags)
	}
	if err := os.WriteFile(bazelrcPath, bazelrcBuf.Bytes(), 0666); err != nil {
		return "", cleanup, err
	}

	// Extract test files for the main repository.
	if err := extractTxtar(mainDir, args.Main); err != nil {
		return "", cleanup, fmt.Errorf("building main workspace: %v", err)
	}

	goRootFilePath, err := runfiles.Rlocation(goRootFile)
	if err != nil {
		return "", cleanup, fmt.Errorf("unknown runfile %s: %v", goRootFile, err)
	}
	// TODO: This is only necessary because of https://github.com/golang/go/issues/59924.
	// Resolving is best effort: Bazel 9 doesn't materialize a runfiles symlink
	// tree on Windows, where the path is already the real one.
	if goRootFileRealPath, err := filepath.EvalSymlinks(goRootFilePath); err == nil {
		goRootFilePath = goRootFileRealPath
	}
	goSDKPath := strings.ReplaceAll(filepath.Dir(goRootFilePath), "\\", "\\\\")

	// If there's no MODULE.bazel file, create one.
	{
		moduleBazelPath := filepath.Join(mainDir, "MODULE.bazel")
		if _, err = os.Stat(moduleBazelPath); err == nil {
			if args.ModuleFileSuffix != "" {
				return "", cleanup, fmt.Errorf("ModuleFileSuffix set but MODULE.bazel exists")
			}
			return mainDir, cleanup, nil
		}
		var w *os.File
		w, err = os.Create(moduleBazelPath)
		if err != nil {
			return "", cleanup, err
		}
		defer func() {
			if cerr := w.Close(); err == nil && cerr != nil {
				err = cerr
			}
		}()
		info := moduleFileTemplateInfo{
			TestedModuleName:     testedModuleName,
			TestedModuleRepoName: testedModuleRepoName,
			TestedModulePath:     strings.ReplaceAll(testedRepoDir, "\\", "\\\\"),
			GoSDKPath:            goSDKPathOrEmpty(args, goSDKPath),
			RulesCCVersion:       rulesCCVersion(),
			Nogo:                 args.Nogo,
			NogoIncludes:         args.NogoIncludes,
			NogoExcludes:         args.NogoExcludes,
			Suffix:               args.ModuleFileSuffix,
		}
		if err := defaultModuleBazelTpl.Execute(w, info); err != nil {
			return "", cleanup, err
		}
	}

	return mainDir, cleanup, nil
}

// goSDKPathOrEmpty returns the empty string for tests that declare their own Go
// SDK, which makes the module file template omit the wrapped host SDK.
func goSDKPathOrEmpty(args Args, goSDKPath string) string {
	if args.SkipHostGoSDK {
		return ""
	}
	return goSDKPath
}

// rulesCCVersion picks the oldest rules_cc that works with the Bazel version
// under test. Bazel 7 cannot load rules_cc 0.2.14 and later, which introduced
// the compatibility proxy, while the C++ toolchain of releases before 0.2.18
// emits a malformed -fuse-ld flag on macOS. Bazel 9 selects a new enough
// version through its own dependencies either way.
func rulesCCVersion() string {
	if bazelMajorVersion() >= 8 {
		return "0.2.18"
	}
	return "0.2.13"
}

// BazelMajorVersion returns the major version of the bazel binary the test runs
// against, or 0 if it can't be determined. Tests use it to pass flags that only
// exist in some versions.
func BazelMajorVersion() int {
	return bazelMajorVersion()
}

// bazelMajorVersion returns the major version of the selected Bazel binary, or 0
// if it can't be determined.
func bazelMajorVersion() int {
	// --version can't be combined with other startup options, so this doesn't
	// go through BazelCmd.
	cmd := bazelCommand("--version")
	out, err := cmd.Output()
	if err != nil {
		return 0
	}
	// The output has the form "bazel <version>".
	fields := strings.Fields(string(out))
	if len(fields) < 2 {
		return 0
	}
	major, _, _ := strings.Cut(fields[1], ".")
	n, err := strconv.Atoi(major)
	if err != nil {
		return 0
	}
	return n
}

func extractTxtar(dir, txt string) error {
	ar := txtar.Parse([]byte(txt))
	for _, f := range ar.Files {
		if parentDir := filepath.Dir(f.Name); parentDir != "." {
			if err := os.MkdirAll(filepath.Join(dir, parentDir), 0777); err != nil {
				return err
			}
		}
		if err := os.WriteFile(filepath.Join(dir, f.Name), f.Data, 0666); err != nil {
			return err
		}
	}
	return nil
}

// Picks out the first "name = ..." attribute in a WORKSPACE or MODULE.bazel file.
var nameRe = regexp.MustCompile(`(?m)^(?:\s*|workspace\(|module\()name\s*=\s*("[^"]*"|'[^']*')\s*,?\s*\)?\s*$`)

func loadName(bazelFilePath string) (string, error) {
	content, err := os.ReadFile(bazelFilePath)
	if err != nil {
		return "", err
	}
	match := nameRe.FindSubmatchIndex(content)
	if match == nil {
		return "", fmt.Errorf("%s: name not set", bazelFilePath)
	}
	name := string(content[match[2]+1 : match[3]-1])
	if name == "" {
		return "", fmt.Errorf("%s: name is empty", bazelFilePath)
	}
	return name, nil
}

type moduleFileTemplateInfo struct {
	TestedModuleName     string
	TestedModuleRepoName string
	TestedModulePath     string
	GoSDKPath            string
	RulesCCVersion       string
	Nogo                 string
	NogoIncludes         []string
	NogoExcludes         []string
	Suffix               string
}

var defaultModuleBazelTpl = template.Must(template.New("").Parse(`
bazel_dep(name = "{{.TestedModuleName}}", repo_name = "{{.TestedModuleRepoName}}")
local_path_override(
    module_name = "{{.TestedModuleName}}",
    path = "{{.TestedModulePath}}",
)

# Provided by default so that fixtures don't have to declare them themselves.
bazel_dep(name = "platforms", version = "1.1.0")
bazel_dep(name = "rules_cc", version = "{{.RulesCCVersion}}")

# GO_BAZEL_TEST_BAZELFLAGS refers to this repository on Windows, where the
# mingw toolchain has to be requested explicitly.
_cc_configure = use_extension("@rules_cc//cc:extensions.bzl", "cc_configure_extension")
use_repo(_cc_configure, "local_config_cc")

{{if .GoSDKPath}}
_new_local_repository = use_repo_rule("@bazel_tools//tools/build_defs/repo:local.bzl", "new_local_repository")

_new_local_repository(
    name = "local_go_sdk",
    path = "{{.GoSDKPath}}",
    build_file_content = "",
)

# The extension proxy is named to not collide with a "go_sdk" the suffix may
# declare itself.
_host_go_sdk = use_extension("@io_bazel_rules_go//go:extensions.bzl", "go_sdk")
_host_go_sdk.wrap(
    root_file = "@local_go_sdk//:ROOT",
)

# The wrap tag has no name attribute; "main___wrap_0" is what the go_sdk
# extension derives for the first wrap tag of a root module. Expose it as
# @go_sdk, which is the name fixtures refer to it by.
use_repo(_host_go_sdk, go_sdk = "main___wrap_0")
{{if .Nogo}}
# Custom nogo analyzers need the analysis framework, which is provided by the
# golang.org/x/tools module fetched via gazelle's go_deps below.
bazel_dep(name = "gazelle", version = "0.51.3")

_nogo_go_deps = use_extension("@gazelle//:extensions.bzl", "go_deps")
_nogo_go_deps.module(
    path = "golang.org/x/tools",
    sum = "h1:qIpSLOxeCYGg9TrcJokLBG4KFA6d795g0xkBkiESGlo=",
    version = "v0.34.0",
)
use_repo(_nogo_go_deps, "org_golang_x_tools")

_host_go_sdk.nogo(
    nogo = "{{.Nogo}}",
    {{ if .NogoIncludes }}
    includes = [
    {{range .NogoIncludes }}
        "{{ . }}",
    {{ end }}
    ],
    {{ end}}
    {{ if .NogoExcludes }}
    excludes = [
    {{range .NogoExcludes }}
        "{{ . }}",
    {{ end }}
    ],
    {{ end}}
)
{{end}}
{{end}}
{{.Suffix}}
`))

func copyOrLink(dstPath, srcPath string) error {
	if err := os.MkdirAll(filepath.Dir(dstPath), 0777); err != nil {
		return err
	}

	copy := func(dstPath, srcPath string) (err error) {
		src, err := os.Open(srcPath)
		if err != nil {
			return err
		}
		defer src.Close()

		dst, err := os.Create(dstPath)
		if err != nil {
			return err
		}
		defer func() {
			if cerr := dst.Close(); err == nil && cerr != nil {
				err = cerr
			}
		}()

		_, err = io.Copy(dst, src)
		return err
	}

	if runtime.GOOS == "windows" {
		return copy(dstPath, srcPath)
	}
	absSrcPath, err := filepath.Abs(srcPath)
	if err != nil {
		return err
	}
	return os.Symlink(absSrcPath, dstPath)
}
