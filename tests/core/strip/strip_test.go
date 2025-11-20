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

package strip_test

import (
	"bytes"
	"errors"
	"fmt"
	"os/exec"
	"strings"
	"testing"

	"github.com/bazelbuild/rules_go/go/tools/bazel_testing"
)

func TestMain(m *testing.M) {
	bazel_testing.TestMain(m, bazel_testing.Args{
		Main: `
-- BUILD.bazel --
load("@io_bazel_rules_go//go:def.bzl", "go_binary", "go_library", "go_test")

go_library(
    name = "strip_lib",
    srcs = ["strip.go"],
)

go_binary(
    name = "strip",
    embed = [":strip_lib"],
)

go_test(
    name = "strip_test",
    srcs = ["strip_test.go"],
    embed = [":strip_lib"],
)
-- strip.go --
package main

import (
	"debug/elf"
	"debug/macho"
	"debug/pe"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"regexp"
	"runtime"
	"runtime/debug"
	"strings"
)

// Used to disable checking that the length of the captured stack trace exactly
// matches the expected length. This allows the test for testing cases to not
// encode so many details about internals of the go testing package.
var checkStackTraceLenEq = flag.Bool("check-len-eq", true, "")
var wantStrip = flag.Bool("wantstrip", false, "")

func main() {
	flag.Parse()
	stackTrace, err := panicAndRecover()
	if err != nil {
		panic(err)
	}
	gotStackTrace := strings.Split(stackTrace, "\n")
	if *checkStackTraceLenEq {
		if len(gotStackTrace) != len(wantStackTrace) {
			panic(fmt.Sprintf("got %d lines of stack trace, want %d", len(gotStackTrace), len(wantStackTrace)))
		}
	} else {
		if len(gotStackTrace) < len(wantStackTrace) {
			panic(fmt.Sprintf("got %d lines of stack trace, want at least %d", len(gotStackTrace), len(wantStackTrace)))
		}
	}
	for i := range wantStackTrace {
		expectedLine := regexp.MustCompile(wantStackTrace[i])
		if !expectedLine.MatchString(gotStackTrace[i]) {
			panic(fmt.Sprintf("got unexpected stack trace line %q at index %d", gotStackTrace[i], i))
		}
	}
	stripped, err := isStripped()
	if err != nil {
		panic(err)
	}
	if stripped != *wantStrip {
		panic(fmt.Sprintf("got stripped=%t, want stripped=%t", stripped, *wantStrip))
	}
}

func panicAndRecover() (stackTrace string, err error) {
	defer func() {
		if r := recover(); r != nil {
			stackTrace = string(debug.Stack())
		}
	}()
	panic("test")
	return "", errors.New("should not reach here")
}

func isStripped() (bool, error) {
	ownLocation, err := os.Executable()
	if err != nil {
		return false, err
	}
	ownBinary, err := os.Open(ownLocation)
	if err != nil {
		return false, err
	}
	defer ownBinary.Close()
	switch runtime.GOOS {
	case "darwin":
		return isStrippedMachO(ownBinary)
	case "linux":
		return isStrippedElf(ownBinary)
	case "windows":
		return isStrippedPE(ownBinary)
	default:
		return false, fmt.Errorf("unsupported OS: %s", runtime.GOOS)
	}
}

func isStrippedMachO(f io.ReaderAt) (bool, error) {
	macho, err := macho.NewFile(f)
	if err != nil {
		return false, err
	}
	gotDwarf := macho.Segment("__DWARF") != nil
	gotDebugInfo := macho.Section("__zdebug_info") != nil
	if gotDwarf != gotDebugInfo {
		return false, fmt.Errorf("inconsistent stripping: gotDwarf=%v, gotDebugInfo=%v", gotDwarf, gotDebugInfo)
	}
	return !gotDwarf, nil
}

func isStrippedElf(f io.ReaderAt) (bool, error) {
	elf, err := elf.NewFile(f)
	if err != nil {
		return false, err
	}
	var gotSymtab bool
	for _, section := range elf.Sections {
		if section.Name == ".symtab" {
			gotSymtab = true
			break
		}
	}
	return !gotSymtab, nil
}


func isStrippedPE(f io.ReaderAt) (bool, error) {
	pe, err := pe.NewFile(f)
	if err != nil {
		return false, err
	}
	// When the COFF symbol table is stripped the linker clears both the
	// pointer and the symbol count in the PE header. That mirrors what Go's
	// toolchain emits for PIE binaries.
	if pe.FileHeader.PointerToSymbolTable == 0 || pe.FileHeader.NumberOfSymbols == 0 {
		return true, nil
	}
	// Some toolchains leave the header populated but drop the table contents.
	// Fall back to checking whether the parsed table still has entries.
	return len(pe.COFFSymbols) == 0, nil
}


` + embedWantedStackTraces() + `
-- strip_test.go --
package main

import "testing"

func TestStrip(t *testing.T) {
	defer func(prev []string) { wantStackTrace = prev }(wantStackTrace)
	wantStackTrace = wantTestStackTrace
	main()
}
` + embedWantedTestStackTrace(),
	})
}

func Test(t *testing.T) {
	type testCase struct {
		desc, stripFlag, compilationMode string
		wantStrip                        bool
	}
	testArgs := func(test testCase, bazelCmd string) []string {
		args := []string{bazelCmd}
		if len(test.stripFlag) > 0 {
			args = append(args, "--strip", test.stripFlag)
		}
		if len(test.compilationMode) > 0 {
			args = append(args, "--compilation_mode", test.compilationMode)
		}
		stripFlag := fmt.Sprintf("-wantstrip=%v", test.wantStrip)
		switch bazelCmd {
		case "test":
			stripFlag = fmt.Sprintf("--test_arg=%s", stripFlag)
			checkLenFlag := "--test_arg=-check-len-eq=false"
			args = append(args, "//:strip_test", stripFlag, checkLenFlag)
		case "run":
			args = append(args, "//:strip", "--", stripFlag)
		default:
			panic(fmt.Sprintf("unknown command: %s", bazelCmd))
		}
		return args
	}
	cases := []testCase{
		{
			desc:      "run_auto",
			wantStrip: true,
		},
		{
			desc:            "run_fastbuild",
			compilationMode: "fastbuild",
			wantStrip:       true,
		},
		{
			desc:            "run_dbg",
			compilationMode: "dbg",
		},
		{
			desc:            "run_opt",
			compilationMode: "opt",
		},
		{
			desc:      "run_always",
			stripFlag: "always",
			wantStrip: true,
		},
		{
			desc:            "run_always_opt",
			stripFlag:       "always",
			compilationMode: "opt",
			wantStrip:       true,
		},
		{
			desc:      "run_never",
			stripFlag: "never",
		},
		{
			desc:            "run_sometimes_fastbuild",
			stripFlag:       "sometimes",
			compilationMode: "fastbuild",
			wantStrip:       true,
		},
		{
			desc:            "run_sometimes_dbg",
			stripFlag:       "sometimes",
			compilationMode: "dbg",
		},
		{
			desc:            "run_sometimes_opt",
			stripFlag:       "sometimes",
			compilationMode: "opt",
		},
	}
	run := func(t *testing.T, args []string) {
		cmd := bazel_testing.BazelCmd(args...)
		stderr := &bytes.Buffer{}
		cmd.Stderr = stderr
		t.Logf("running: bazel %s", strings.Join(args, " "))
		if err := cmd.Run(); err != nil {
			var xerr *exec.ExitError
			if !errors.As(err, &xerr) {
				t.Fatalf("unexpected error: %v", err)
			}
			if xerr.ExitCode() == bazel_testing.BUILD_FAILURE {
				t.Fatalf("unexpected build failure: %v\nstderr:\n%s", err, stderr.Bytes())
				return
			} else if xerr.ExitCode() == bazel_testing.TESTS_FAILED {
				t.Fatalf("error running %s:\n%s", strings.Join(cmd.Args, " "), stderr.Bytes())
			} else {
				t.Fatalf("unexpected error: %v\nstderr:\n%s", err, stderr.Bytes())
			}
		}
	}
	for _, bazelCmd := range []string{"run", "test"} {
		t.Run(bazelCmd, func(t *testing.T) {
			for _, test := range cases {
				t.Run(test.desc, func(t *testing.T) {
					run(t, testArgs(test, bazelCmd))
				})
			}
		})
	}
}

func embedWantedStackTraces() string {
	return embedStringSliceVar("wantStackTrace", wantStackTrace)
}

var wantStackTrace = []string{
	`^goroutine \d+ \[running\]:$`,
	`^runtime/debug\.Stack\(\)$`,
	`^	GOROOT/src/runtime/debug/stack\.go:\d+ \+0x[0-9a-f]+$`,
	`^main\.panicAndRecover\.func1\(\)$`,
	`^	strip\.go:\d+ \+0x[0-9a-f]+$`,
	`^panic\({0x[0-9a-f]+\?*, 0x[0-9a-f]+\?*}\)$`,
	`^	GOROOT/src/runtime/panic\.go:\d+ \+0x[0-9a-f]+$`,
	`^main\.panicAndRecover\(\)$`,
	`^	strip\.go:\d+ \+0x[0-9a-f]+$`,
	`^main\.main\(\)$`,
	`^	strip\.go:\d+ \+0x[0-9a-f]+$`,
	`^$`,
}

func embedWantedTestStackTrace() string {
	return embedStringSliceVar("wantTestStackTrace", wantTestStackTrace)
}

var wantTestStackTrace = replaceStrings(
	wantStackTrace[:len(wantStackTrace)-1], // remove the final empty line.
	"^main", "^strip_test",
)

func replaceStrings(data []string, old, new string) []string {
	ret := make([]string, len(data))
	for i, s := range data {
		ret[i] = strings.Replace(s, old, new, 1)
	}
	return ret
}

func embedStringSliceVar(name string, data []string) string {
	buf := &bytes.Buffer{}
	fmt.Fprintf(buf, "var %s = []string{\n", name)
	for _, s := range data {
		fmt.Fprintf(buf, "`%s`,\n", s)
	}
	fmt.Fprintln(buf, "}")
	return buf.String()
}
