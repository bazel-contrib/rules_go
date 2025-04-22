// Copyright 2017 The Bazel Authors. All rights reserved.
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
	"errors"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
)

var (
	// cgoEnvVars is the list of all cgo environment variable
	cgoEnvVars = []string{"CGO_CFLAGS", "CGO_CXXFLAGS", "CGO_CPPFLAGS", "CGO_LDFLAGS"}
	// cgoAbsEnvFlags are all the flags that need absolute path in cgoEnvVars
	cgoAbsEnvFlags = []string{"-I", "-L", "-isysroot", "-isystem", "-iquote", "-include", "-imacros", "-gcc-toolchain", "--sysroot", "-resource-dir", "-fsanitize-blacklist", "-fsanitize-ignorelist"}
	// cgoAbsPlaceholder is placed in front of flag values that must be absolutized
	cgoAbsPlaceholder = "__GO_BAZEL_CC_PLACEHOLDER__"
)

// env holds a small amount of Go environment and toolchain information
// which is common to multiple builders. Most Bazel-agnostic build information
// is collected in go/build.Default though.
//
// See ./README.rst for more information about handling arguments and
// environment variables.
type env struct {
	// sdk is the path to the Go SDK, which contains tools for the host
	// platform. This may be different than GOROOT.
	sdk string

	// goroot is set as the value of GOROOT if non-empty.
	goroot string

	// installSuffix is the name of the directory below GOROOT/pkg that contains
	// the .a files for the standard library we should build against.
	// For example, linux_amd64_race.
	installSuffix string

	// verbose indicates whether subprocess command lines should be printed.
	verbose bool

	// workDirPath is a temporary work directory. It is created lazily.
	workDirPath string

	shouldPreserveWorkDir bool
}

// envFlags registers flags common to multiple builders and returns an env
// configured with those flags.
func envFlags(flags *flag.FlagSet) *env {
	env := &env{}
	flags.StringVar(&env.sdk, "sdk", "", "Path to the Go SDK.")
	flags.StringVar(&env.goroot, "goroot", "", "The value to set for GOROOT.")
	flags.Var(&tagFlag{}, "tags", "List of build tags considered true.")
	flags.StringVar(&env.installSuffix, "installsuffix", "", "Standard library under GOROOT/pkg")
	flags.BoolVar(&env.verbose, "v", false, "Whether subprocess command lines should be printed")
	flags.BoolVar(&env.shouldPreserveWorkDir, "work", false, "if true, the temporary work directory will be preserved")
	return env
}

// checkFlagsAndSetGoroot checks whether env flags were set to valid values and sets GOROOT.
// checkFlagsAndSetGoroot should be called after parsing flags.
func (e *env) checkFlagsAndSetGoroot() error {
	if e.sdk == "" {
		return errors.New("-sdk was not set")
	}
	if e.goroot != "" {
		err := os.Setenv("GOROOT", e.goroot)
		if err != nil {
			return err
		}
	}
	return nil
}

// workDir returns a path to a temporary work directory. The same directory
// is returned on multiple calls. The caller is responsible for cleaning
// up the work directory by calling cleanup.
func (e *env) workDir() (path string, cleanup func(), err error) {
	if e.workDirPath != "" {
		return e.workDirPath, func() {}, nil
	}
	// Keep the stem "rules_go_work" in sync with reproducible_binary_test.go.
	e.workDirPath, err = ioutil.TempDir("", "rules_go_work-")
	if err != nil {
		return "", func() {}, err
	}
	if e.verbose {
		log.Printf("WORK=%s\n", e.workDirPath)
	}
	if e.shouldPreserveWorkDir {
		cleanup = func() {}
	} else {
		cleanup = func() { os.RemoveAll(e.workDirPath) }
	}
	return e.workDirPath, cleanup, nil
}

// goTool returns a slice containing the path to an executable at
// $GOROOT/pkg/$GOOS_$GOARCH/$tool and additional arguments.
func (e *env) goTool(tool string, args ...string) []string {
	platform := fmt.Sprintf("%s_%s", runtime.GOOS, runtime.GOARCH)
	toolPath := filepath.Join(e.sdk, "pkg", "tool", platform, tool)
	if runtime.GOOS == "windows" {
		toolPath += ".exe"
	}
	return append([]string{toolPath}, args...)
}

// goCmd returns a slice containing the path to the go executable
// and additional arguments.
func (e *env) goCmd(cmd string, args ...string) []string {
	exe := filepath.Join(e.sdk, "bin", "go")
	if runtime.GOOS == "windows" {
		exe += ".exe"
	}
	return append([]string{exe, cmd}, args...)
}

// runCommand executes a subprocess that inherits stdout, stderr, and the
// environment from this process.
func (e *env) runCommand(args []string) error {
	cmd := exec.Command(args[0], args[1:]...)
	// Redirecting stdout to stderr. This mirrors behavior in the go command:
	// https://go.googlesource.com/go/+/refs/tags/go1.15.2/src/cmd/go/internal/work/exec.go#1958
	buf := &bytes.Buffer{}
	cmd.Stdout = buf
	cmd.Stderr = buf
	err := runAndLogCommand(cmd, e.verbose)
	os.Stderr.Write(relativizePaths(buf.Bytes()))
	return err
}

// runCommandToFile executes a subprocess and writes stdout/stderr to the given
// writers.
func (e *env) runCommandToFile(out, err io.Writer, args []string) error {
	cmd := exec.Command(args[0], args[1:]...)
	cmd.Stdout = out
	cmd.Stderr = err
	return runAndLogCommand(cmd, e.verbose)
}

func symlinkBuilderCC() (func(), string, error) {
	absPath, err := filepath.Abs(os.Args[0])
	if err != nil {
		return nil, "", err
	}

	dirname, err := os.MkdirTemp("", "")
	if err != nil {
		return nil, "", err
	}

	// re-use the file extension if there is one (mostly for .exe)
	builderPath := filepath.Join(dirname, "builder-cc"+filepath.Ext(os.Args[0]))

	if err := os.Symlink(absPath, builderPath); err != nil {
		_ = os.RemoveAll(dirname)
		return nil, "", err
	}

	return func() { _ = os.RemoveAll(dirname) }, builderPath, nil
}

var symlinkBuilderName = regexp.MustCompile(`.*builder-(\w+)(\.exe)?$`)

// verbFromName parses the builder verb from the builder name so
// that the builder+verb can be invoked in a way that allows the verb
// to be part of the program name, rather than requring it to be in the
// arg list. this makes it easier to set a program like `builder cc` as
// an environment variable in a way that is more compatible
func verbFromName(arg0 string) string {
	matches := symlinkBuilderName.FindAllStringSubmatch(arg0, 1)
	if len(matches) != 1 {
		return ""
	}

	if len(matches[0]) < 2 {
		return ""
	}

	return matches[0][1]
}

// absCCLinker sets the target of the `-extld` to a name which `verbFromName`
// can extract correctly, allowing go tool link to be able to use the `builder cc` wrapper
func absCCLinker(argList []string) (func(), error) {
	extldIndex := -1
	for i, arg := range argList {
		if arg == "-extld" && i+1 < len(argList) {
			extldIndex = i + 1
			break
		}
	}
	if extldIndex < 0 {
		// if we don't find extld just return
		// a noop cleanup function
		return func() {}, nil
	}

	cleanup, buildercc, err := symlinkBuilderCC()
	if err != nil {
		return nil, err
	}

	err = os.Setenv("GO_CC", argList[extldIndex])
	if err != nil {
		cleanup()
		return nil, err
	}

	err = os.Setenv("GO_CC_ROOT", abs("."))
	if err != nil {
		cleanup()
		return nil, err
	}

	argList[extldIndex] = buildercc

	return cleanup, nil
}

// absCCCompiler modifies CGO flags to workaround relative paths.
// Because go is having its own sandbox, all CGO flags should use
// absolute paths. However, CGO flags are embedded in the output
// so we cannot use absolute paths directly. Instead, use a placeholder
// for the absolute path and we replace CC with this builder so that
// we can expand the placeholder later.
func absCCCompiler(envNameList []string, argList []string) error {
	err := os.Setenv("GO_CC", os.Getenv("CC"))
	if err != nil {
		return err
	}
	err = os.Setenv("GO_CC_ROOT", abs("."))
	if err != nil {
		return err
	}
	err = os.Setenv("CC", abs(os.Args[0])+" cc")
	if err != nil {
		return err
	}
	for _, envName := range envNameList {
		splitedEnv := strings.Fields(os.Getenv(envName))
		transformArgs(splitedEnv, argList, func(s string) string {
			if filepath.IsAbs(s) {
				return s
			}
			return cgoAbsPlaceholder + s
		})
		if err := os.Setenv(envName, strings.Join(splitedEnv, " ")); err != nil {
			return err
		}
	}
	return nil
}

func runAndLogCommand(cmd *exec.Cmd, verbose bool) error {
	if verbose {
		fmt.Fprintln(os.Stderr, formatCommand(cmd))
	}
	cleanup := passLongArgsInResponseFiles(cmd)
	defer cleanup()
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("error running subcommand %s: %w", cmd.Path, err)
	}
	return nil
}

// expandParamsFiles looks for arguments in args of the form
// "-param=filename". When it finds these arguments it reads the file "filename"
// and replaces the argument with its content.
// It returns the expanded arguments as well as a bool that is true if any param
// files have been passed.
func expandParamsFiles(args []string) ([]string, bool, error) {
	var paramsIndices []int
	for i, arg := range args {
		if strings.HasPrefix(arg, "-param=") {
			paramsIndices = append(paramsIndices, i)
		}
	}
	if len(paramsIndices) == 0 {
		return args, false, nil
	}
	var expandedArgs []string
	last := 0
	for _, pi := range paramsIndices {
		expandedArgs = append(expandedArgs, args[last:pi]...)
		last = pi + 1

		fileName := args[pi][len("-param="):]
		fileArgs, err := readParamsFile(fileName)
		if err != nil {
			return nil, true, err
		}
		expandedArgs = append(expandedArgs, fileArgs...)
	}
	expandedArgs = append(expandedArgs, args[last:]...)
	return expandedArgs, true, nil
}

// readParamsFiles parses a Bazel params file in "shell" format. The file
// should contain one argument per line. Arguments may be quoted with single
// quotes. All characters within quoted strings are interpreted literally
// including newlines and excepting single quotes. Characters outside quoted
// strings may be escaped with a backslash.
func readParamsFile(name string) ([]string, error) {
	data, err := ioutil.ReadFile(name)
	if err != nil {
		return nil, err
	}

	var args []string
	var arg []byte
	quote := false
	escape := false
	for p := 0; p < len(data); p++ {
		b := data[p]
		switch {
		case escape:
			arg = append(arg, b)
			escape = false

		case b == '\'':
			quote = !quote

		case !quote && b == '\\':
			escape = true

		case !quote && b == '\n':
			args = append(args, string(arg))
			arg = arg[:0]

		default:
			arg = append(arg, b)
		}
	}
	if quote {
		return nil, fmt.Errorf("unterminated quote")
	}
	if escape {
		return nil, fmt.Errorf("unterminated escape")
	}
	if len(arg) > 0 {
		args = append(args, string(arg))
	}
	return args, nil
}

// writeParamsFile formats a list of arguments in Bazel's "shell" format and writes
// it to a file.
func writeParamsFile(path string, args []string) error {
	buf := new(bytes.Buffer)
	for _, arg := range args {
		if !strings.ContainsAny(arg, "'\n\\") {
			fmt.Fprintln(buf, arg)
			continue
		}
		buf.WriteByte('\'')
		for _, r := range arg {
			if r == '\'' {
				buf.WriteString(`'\''`)
			} else {
				buf.WriteRune(r)
			}
		}
		buf.WriteString("'\n")
	}
	return ioutil.WriteFile(path, buf.Bytes(), 0666)
}

// splitArgs splits a list of command line arguments into two parts: arguments
// that should be interpreted by the builder (before "--"), and arguments
// that should be passed through to the underlying tool (after "--").
func splitArgs(args []string) (builderArgs []string, toolArgs []string) {
	for i, arg := range args {
		if arg == "--" {
			return args[:i], args[i+1:]
		}
	}
	return args, nil
}

// abs returns the absolute representation of path. Some tools/APIs require
// absolute paths to work correctly. Most notably, golang on Windows cannot
// handle relative paths to files whose absolute path is > ~250 chars, while
// it can handle absolute paths. See http://goo.gl/eqeWjm.
//
// Note that strings that begin with "__BAZEL_" are not absolutized. These are
// used on macOS for paths that the compiler wrapper (wrapped_clang) is
// supposed to know about.
func abs(path string) string {
	if strings.HasPrefix(path, "__BAZEL_") {
		return path
	}

	if abs, err := filepath.Abs(path); err != nil {
		return path
	} else {
		return abs
	}
}

// absArgs applies abs to strings that appear in args. Only paths that are
// part of options named by flags are modified.
func absArgs(args []string, flags []string) {
	transformArgs(args, flags, abs)
}

// transformArgs applies fn to strings that appear in args. Only paths that are
// part of options named by flags are modified.
func transformArgs(args []string, flags []string, fn func(string) string) {
	absNext := false
	for i := range args {
		if absNext {
			args[i] = fn(args[i])
			absNext = false
			continue
		}
		for _, f := range flags {
			if !strings.HasPrefix(args[i], f) {
				continue
			}
			possibleValue := args[i][len(f):]
			if len(possibleValue) == 0 {
				absNext = true
				break
			}
			separator := ""
			if possibleValue[0] == '=' {
				possibleValue = possibleValue[1:]
				separator = "="
			}
			args[i] = fmt.Sprintf("%s%s%s", f, separator, fn(possibleValue))
			break
		}
	}
}

// relativizePaths converts absolute paths found in the given output string to
// relative, if they are within the working directory.
func relativizePaths(output []byte) []byte {
	dir, err := os.Getwd()
	if dir == "" || err != nil {
		return output
	}
	dirBytes := make([]byte, len(dir), len(dir)+1)
	copy(dirBytes, dir)
	if bytes.HasSuffix(dirBytes, []byte{filepath.Separator}) {
		return bytes.ReplaceAll(output, dirBytes, nil)
	}

	// This is the common case.
	// Replace "$CWD/" with "" and "$CWD" with "."
	dirBytes = append(dirBytes, filepath.Separator)
	output = bytes.ReplaceAll(output, dirBytes, nil)
	dirBytes = dirBytes[:len(dirBytes)-1]
	return bytes.ReplaceAll(output, dirBytes, []byte{'.'})
}

// formatCommand formats cmd as a string that can be pasted into a shell.
// Spaces in environment variables and arguments are escaped as needed.
func formatCommand(cmd *exec.Cmd) string {
	quoteIfNeeded := func(s string) string {
		if strings.IndexByte(s, ' ') < 0 {
			return s
		}
		return strconv.Quote(s)
	}
	quoteEnvIfNeeded := func(s string) string {
		eq := strings.IndexByte(s, '=')
		if eq < 0 {
			return s
		}
		key, value := s[:eq], s[eq+1:]
		if strings.IndexByte(value, ' ') < 0 {
			return s
		}
		return fmt.Sprintf("%s=%s", key, strconv.Quote(value))
	}
	var w bytes.Buffer
	environ := cmd.Env
	if environ == nil {
		environ = os.Environ()
	}
	for _, e := range environ {
		fmt.Fprintf(&w, "%s \\\n", quoteEnvIfNeeded(e))
	}

	sep := ""
	for _, arg := range cmd.Args {
		fmt.Fprintf(&w, "%s%s", sep, quoteIfNeeded(arg))
		sep = " "
	}
	return w.String()
}

// passLongArgsInResponseFiles modifies cmd such that, for
// certain programs, long arguments are passed in "response files", a
// file on disk with the arguments, with one arg per line. An actual
// argument starting with '@' means that the rest of the argument is
// a filename of arguments to expand.
//
// See https://github.com/golang/go/issues/18468 (Windows) and
// https://github.com/golang/go/issues/37768 (Darwin).
func passLongArgsInResponseFiles(cmd *exec.Cmd) (cleanup func()) {
	cleanup = func() {} // no cleanup by default
	var argLen int
	for _, arg := range cmd.Args {
		argLen += len(arg)
	}
	// If we're not approaching 32KB of args, just pass args normally.
	// (use 30KB instead to be conservative; not sure how accounting is done)
	if !useResponseFile(cmd.Path, argLen) {
		return
	}
	tf, err := ioutil.TempFile("", "args")
	if err != nil {
		log.Fatalf("error writing long arguments to response file: %v", err)
	}
	cleanup = func() { os.Remove(tf.Name()) }
	var buf bytes.Buffer
	for _, arg := range cmd.Args[1:] {
		// Slashes need to be doubled for escaping
		escaped_arg := strings.ReplaceAll(arg, "\\", "\\\\")
		// Newlines too, so that they don't get split when read
		escaped_arg = strings.ReplaceAll(escaped_arg, "\n", "\\n")
		fmt.Fprintf(&buf, "%s\n", escaped_arg)
	}
	if _, err := tf.Write(buf.Bytes()); err != nil {
		tf.Close()
		cleanup()
		log.Fatalf("error writing long arguments to response file: %v", err)
	}
	if err := tf.Close(); err != nil {
		cleanup()
		log.Fatalf("error writing long arguments to response file: %v", err)
	}
	cmd.Args = []string{cmd.Args[0], "@" + tf.Name()}
	return cleanup
}

// quotePathIfNeeded quotes path if it contains whitespace and isn't already quoted.
// Use this for paths that will be passed through
// https://github.com/golang/go/blob/06264b740e3bfe619f5e90359d8f0d521bd47806/src/cmd/internal/quoted/quoted.go#L25
func quotePathIfNeeded(path string) string {
	if strings.HasPrefix(path, "\"") || strings.HasPrefix(path, "'") {
		// Assume already quoted
		return path
	}
	// https://github.com/golang/go/blob/06264b740e3bfe619f5e90359d8f0d521bd47806/src/cmd/internal/quoted/quoted.go#L16
	if strings.IndexAny(path, " \t\n\r") < 0 {
		// Does not require quoting
		return path
	}
	// Escaping quotes is not supported, so we can assume path doesn't contain any quotes.
	return "'" + path + "'"
}

func useResponseFile(path string, argLen int) bool {
	// Unless the program uses objabi.Flagparse, which understands
	// response files, don't use response files.
	// TODO: do we need more commands? asm? cgo? For now, no.
	prog := strings.TrimSuffix(filepath.Base(path), ".exe")
	switch prog {
	case "compile", "link":
	default:
		return false
	}
	// Windows has a limit of 32 KB arguments. To be conservative and not
	// worry about whether that includes spaces or not, just use 30 KB.
	// Darwin's limit is less clear. The OS claims 256KB, but we've seen
	// failures with arglen as small as 50KB.
	if argLen > (30 << 10) {
		return true
	}
	return false
}
