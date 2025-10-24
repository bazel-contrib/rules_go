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
	"strings"
	"testing"
	"text/template"

	"github.com/bazelbuild/rules_go/go/runfiles"
	"github.com/bazelbuild/rules_go/go/tools/bazel"
	"github.com/bazelbuild/rules_go/go/tools/internal/txtar"
)

// Set via x_defs.
var goRootFile string

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
	// Main is a text archive containing files in the main workspace.
	// The text archive format is parsed by
	// //go/tools/internal/txtar:go_default_library, which is copied from
	// cmd/go/internal/txtar. If this archive does not contain a WORKSPACE file,
	// a default file will be synthesized.
	Main string

	// Nogo is the nogo target to pass to go_register_toolchains. By default,
	// nogo is not used.
	Nogo string

	// NogoIncludes is the list of targets to include for Nogo linting.
	NogoIncludes []string

	// NogoExcludes is the list of targets to include for Nogo linting.
	NogoExcludes []string

	// WorkspacePrefix is a string that should be inserted at the beginning
	// of the default generated WORKSPACE file.
	WorkspacePrefix string

	// WorkspaceSuffix is a string that should be appended to the end
	// of the default generated WORKSPACE file.
	WorkspaceSuffix string

	// ModuleFileSuffix is a string that should be appended to the end of a
	// default generated MODULE.bazel file. If this is empty, no such file is
	// generated.
	ModuleFileSuffix string

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
// We try to detect the original value in setupWorkspace and set it to that.
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

	if err := os.Chdir(workspaceDir); err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		return
	}
	defer exec.Command("bazel", "shutdown").Run()

	if args.SetUp != nil {
		if err := args.SetUp(); err != nil {
			fmt.Fprintf(os.Stderr, "test provided SetUp method returned error: %v\n", err)
			return
		}
	}

	code = m.Run()
}

// BazelCmd prepares a bazel command for execution. It chooses the correct
// bazel binary based on the environment and sanitizes the environment to
// hide that this code is executing inside a bazel test.
func BazelCmd(args ...string) *exec.Cmd {
	cmd := exec.Command("bazel")
	// --nosystem_rc isn't used here because Bazel may need essential flags set in
	// system rc to be able to work correctly
	// See https://github.com/bazelbuild/rules_go/pull/3969#issuecomment-2220405416
	cmd.Args = append(cmd.Args, "--nohome_rc")
	cmd.Args = append(cmd.Args, args...)
	for _, e := range os.Environ() {
		// Filter environment variables set by the bazel test wrapper script.
		// These confuse recursive invocations of Bazel.
		if strings.HasPrefix(e, "TEST_") || strings.HasPrefix(e, "RUNFILES_") {
			continue
		}
		cmd.Env = append(cmd.Env, e)
	}
	return cmd
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

	// Find a suitable cache directory. We want something persistent where we
	// can store a bazel output base across test runs, even for multiple tests.
	var cacheDir, outBaseDir string
	if tmpDir := os.Getenv("TEST_TMPDIR"); tmpDir != "" {
		// TEST_TMPDIR is set by Bazel's test wrapper. Bazel itself uses this to
		// detect that it's run by a test. When invoked like this, Bazel sets
		// its output base directory to a temporary directory. This wastes a lot
		// of time (a simple test takes 45s instead of 3s). We use TEST_TMPDIR
		// to find a persistent location in the execroot. We won't pass TEST_TMPDIR
		// to bazel in RunBazel.
		tmpDir = filepath.Clean(tmpDir)
		if i := strings.Index(tmpDir, string(os.PathSeparator)+"execroot"+string(os.PathSeparator)); i >= 0 {
			outBaseDir = tmpDir[:i]
			if dir, err := filepath.Abs(filepath.Dir(outBaseDir)); err == nil {
				// Use forward slashes, even on Windows. Bazel's rc file parser
				// reports an error if there are backslashes.
				outputUserRoot = strings.ReplaceAll(dir, `\`, `/`)
			}
			cacheDir = filepath.Join(outBaseDir, "bazel_testing")
		} else {
			cacheDir = filepath.Join(tmpDir, "bazel_testing")
		}
	} else {
		// The test is not invoked by Bazel, so just use the user's cache.
		cacheDir, err = os.UserCacheDir()
		if err != nil {
			return "", cleanup, err
		}
		cacheDir = filepath.Join(cacheDir, "bazel_testing")
	}

	// TODO(jayconrod): any other directories needed for caches?
	execDir := filepath.Join(cacheDir, "bazel_go_test")
	if err := os.RemoveAll(execDir); err != nil {
		return "", cleanup, err
	}
	cleanups = append(cleanups, func() error { return os.RemoveAll(execDir) })

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
	if args.ModuleFileSuffix == "" {
		fmt.Fprintf(bazelrcBuf, "common --noenable_bzlmod\n")
	} else {
		fmt.Fprintf(bazelrcBuf, "common --enable_bzlmod\n")
	}
	if outputUserRoot != "" {
		fmt.Fprintf(bazelrcBuf, "startup --output_user_root=%s\n", outputUserRoot)
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

	// If there's no WORKSPACE file, create one.
	workspacePath := filepath.Join(mainDir, "WORKSPACE")
	if _, err = os.Stat(workspacePath); os.IsNotExist(err) {
		var w *os.File
		w, err = os.Create(workspacePath)
		if err != nil {
			return "", cleanup, err
		}
		defer func() {
			if cerr := w.Close(); err == nil && cerr != nil {
				err = cerr
			}
		}()
		goRootFilePath, err := runfiles.Rlocation(goRootFile)
		if err != nil {
			return "", cleanup, fmt.Errorf("unknown runfile %s: %v", goRootFile, err)
		}
		// TODO: This is only necessary because of https://github.com/golang/go/issues/59924.
		goRootFileRealPath, err := filepath.EvalSymlinks(goRootFilePath)
		if err != nil {
			return "", cleanup, fmt.Errorf("unknown runfile %s: %v", goRootFile, err)
		}
		info := workspaceTemplateInfo{
			TestedModuleRepoName: testedModuleRepoName,
			TestedModulePath:     strings.ReplaceAll(testedRepoDir, "\\", "\\\\"),
			Prefix:               args.WorkspacePrefix,
			Suffix:               args.WorkspaceSuffix,
			Nogo:                 args.Nogo,
			NogoIncludes:         args.NogoIncludes,
			NogoExcludes:         args.NogoExcludes,
			GoSDKPath:            strings.ReplaceAll(filepath.Dir(goRootFileRealPath), "\\", "\\\\"),
		}
		if err := defaultWorkspaceTpl.Execute(w, info); err != nil {
			return "", cleanup, err
		}
	}

	// If a MODULE.bazel file is requested, create one.
	if args.ModuleFileSuffix != "" {
		moduleBazelPath := filepath.Join(mainDir, "MODULE.bazel")
		if _, err = os.Stat(moduleBazelPath); err == nil {
			return "", cleanup, fmt.Errorf("ModuleFileSuffix set but MODULE.bazel exists")
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
			Suffix:               args.ModuleFileSuffix,
		}
		if err := defaultModuleBazelTpl.Execute(w, info); err != nil {
			return "", cleanup, err
		}
	}

	return mainDir, cleanup, nil
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

type workspaceTemplateInfo struct {
	TestedModuleRepoName string
	TestedModulePath     string
	GoSDKPath            string
	Nogo                 string
	NogoIncludes         []string
	NogoExcludes         []string
	Prefix               string
	Suffix               string
}

var defaultWorkspaceTpl = template.Must(template.New("").Parse(`
local_repository(
    name = "{{.TestedModuleRepoName}}",
    path = "{{.TestedModulePath}}",
)

{{.Prefix}}

new_local_repository(
    name = "local_go_sdk",
    path = "{{.GoSDKPath}}",
    build_file_content = "",
)

load("@io_bazel_rules_go//go:deps.bzl", "go_rules_dependencies", "go_register_toolchains", "go_wrap_sdk", "go_register_nogo")

go_rules_dependencies()

go_wrap_sdk(
    name = "go_sdk",
    root_file = "@local_go_sdk//:ROOT",
)

go_register_toolchains()

{{if .Nogo}}
go_register_nogo(
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
	{{ else }}
	excludes = None,
	{{ end}}
)
{{end}}

# Create the host platform repository transitively required by rules_go.
load("@bazel_tools//tools/build_defs/repo:utils.bzl", "maybe")
load("@platforms//host:extension.bzl", "host_platform_repo")

maybe(
	host_platform_repo,
	name = "host_platform",
)

{{.Suffix}}
`))

type moduleFileTemplateInfo struct {
	TestedModuleName     string
	TestedModuleRepoName string
	TestedModulePath     string
	Suffix               string
}

// TODO: Also reuse the current Go SDK as in the WORKSPACE file.
var defaultModuleBazelTpl = template.Must(template.New("").Parse(`
bazel_dep(name = "{{.TestedModuleName}}", repo_name = "{{.TestedModuleRepoName}}")
local_path_override(
    module_name = "{{.TestedModuleName}}",
    path = "{{.TestedModulePath}}",
)
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
