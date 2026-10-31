// Copyright 2026 The Bazel Authors. All rights reserved.
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

package auto_internal_link_test

import (
	"debug/elf"
	"encoding/json"
	"os"
	"runtime"
	"slices"
	"strings"
	"testing"

	"github.com/bazelbuild/rules_go/go/tools/bazel_testing"
)

func TestMain(m *testing.M) {
	bazel_testing.TestMain(m, bazel_testing.Args{
		Main: `
-- src/pure_lib.go --
package purelib

func Greet() string { return "hello" }

-- src/pure_lib_test.go --
package purelib_test

import (
	"testing"
	"example.com/purelib"
)

func TestGreet(t *testing.T) {
	if purelib.Greet() != "hello" {
		t.Fatal("unexpected greeting")
	}
}

-- src/cgo_lib.go --
package cgolib

import "C"

func Answer() int { return 42 }

-- src/mid_lib.go --
package midlib

import "example.com/cgolib"

func DoubleAnswer() int { return cgolib.Answer() * 2 }

-- src/pure_main.go --
package main

import (
	"fmt"
	"example.com/purelib"
)

func main() {
	fmt.Println(purelib.Greet())
}

-- src/cgo_main.go --
package main

import (
	"fmt"
	"example.com/cgolib"
)

func main() {
	fmt.Println(cgolib.Answer())
}

-- src/transitive_cgo_main.go --
package main

import (
	"fmt"
	"example.com/midlib"
)

func main() {
	fmt.Println(midlib.DoubleAnswer())
}

-- src/empty.syso --

-- src/buildinfo_main.go --
package main

import (
	"encoding/json"
	"os"
	"runtime/debug"
)

func main() {
	info, ok := debug.ReadBuildInfo()
	if !ok {
		os.Exit(1)
	}
	settings := make(map[string]string)
	for _, s := range info.Settings {
		settings[s.Key] = s.Value
	}
	json.NewEncoder(os.Stdout).Encode(settings)
}

-- src/asm_lib.go --
package asmlib

func Answer() int { return 42 }

-- src/asm_lib.s --
#include "asm_lib.h"

-- src/asm_lib.h --
#define ANSWER 42

-- src/asm_main.go --
package main

import (
	"fmt"
	"example.com/asmlib"
)

func main() {
	fmt.Println(asmlib.Answer())
}

-- src/select_pure.go --
package selectcgo

func Name() string { return "pure" }

-- src/select_cgo_windows.go --
//go:build windows

package selectcgo

import "C"

func cgoName() string { return "cgo" }

-- src/select_main.go --
package main

import (
	"fmt"
	"example.com/selectcgo"
)

func main() {
	fmt.Println(selectcgo.Name())
}

-- src/dep_on_lib_and_cgo.go --
package deponlibandcgo

import (
	"example.com/cgolib"
	"example.com/purelib"
)

func Both() int { return len(purelib.Greet()) + cgolib.Answer() }

-- src/stub_test.go --
package purelib_test

import (
	"testing"
	"example.com/deponlibandcgo"
)

func TestBoth(t *testing.T) {
	if deponlibandcgo.Both() == 0 {
		t.Fatal("unexpected result")
	}
}

-- src/plugin_host.go --
package main

import (
	"fmt"
	"plugin"
)

func main() {
	_, err := plugin.Open("missing.so")
	fmt.Println(err != nil)
}

-- src/cc_files.bzl --
load("@rules_cc//cc:find_cc_toolchain.bzl", "find_cc_toolchain", "use_cc_toolchain")

def _cc_toolchain_files_impl(ctx):
    out = ctx.actions.declare_file(ctx.label.name + ".txt")
    files = find_cc_toolchain(ctx).all_files.to_list()
    ctx.actions.write(out, "".join([f.path + "\n" for f in files]))
    return [DefaultInfo(files = depset([out]))]

cc_toolchain_files = rule(
    implementation = _cc_toolchain_files_impl,
    fragments = ["cpp"],
    toolchains = use_cc_toolchain(),
)

-- src/BUILD.bazel --
load("@io_bazel_rules_go//go:def.bzl", "go_binary", "go_library", "go_test")
load(":cc_files.bzl", "cc_toolchain_files")

go_library(
    name = "purelib",
    srcs = ["pure_lib.go"],
    importpath = "example.com/purelib",
)

go_test(
    name = "pure_test",
    srcs = ["pure_lib_test.go"],
    embed = [":purelib"],
)

go_library(
    name = "cgolib",
    srcs = ["cgo_lib.go"],
    cgo = True,
    importpath = "example.com/cgolib",
)

go_library(
    name = "midlib",
    srcs = ["mid_lib.go"],
    importpath = "example.com/midlib",
    deps = [":cgolib"],
)

go_binary(
    name = "pure_bin",
    srcs = ["pure_main.go"],
    deps = [":purelib"],
)

go_binary(
    name = "pie_pure_bin",
    srcs = ["pure_main.go"],
    linkmode = "pie",
    deps = [":purelib"],
)

# Uses the default link mode, which is normal on Linux. An explicit
# linkmode = "normal" would build the standard library a second time, so
# static_normal_bin, which sets it, is only analyzed.
go_binary(
    name = "static_bin",
    srcs = ["pure_main.go"],
    static = "on",
    deps = [":purelib"],
)

go_binary(
    name = "static_normal_bin",
    srcs = ["pure_main.go"],
    linkmode = "normal",
    static = "on",
    deps = [":purelib"],
)

go_binary(
    name = "static_pie_bin",
    srcs = ["pure_main.go"],
    linkmode = "pie",
    static = "on",
    deps = [":purelib"],
)

go_binary(
    name = "static_cgo_bin",
    srcs = ["cgo_main.go"],
    static = "on",
    deps = [":cgolib"],
)

go_binary(
    name = "static_pure_off_bin",
    srcs = ["pure_main.go"],
    pure = "off",
    static = "on",
    deps = [":purelib"],
)

# Analysis cannot see that these binaries import the plugin package.
go_binary(
    name = "plugin_host_bin",
    srcs = ["plugin_host.go"],
)

go_binary(
    name = "plugin_host_off_bin",
    srcs = ["plugin_host.go"],
    pure = "off",
)

# A genrule runs tool_bin on the execution platform, so Bazel builds it in the
# exec configuration.
go_binary(
    name = "tool_bin",
    srcs = ["pure_main.go"],
    deps = [":purelib"],
)

genrule(
    name = "tool_gen",
    outs = ["tool_gen.txt"],
    cmd = "$(execpath :tool_bin) > $@",
    tools = [":tool_bin"],
)

go_binary(
    name = "race_bin",
    srcs = ["pure_main.go"],
    race = "on",
    deps = [":purelib"],
)

go_binary(
    name = "msan_bin",
    srcs = ["pure_main.go"],
    msan = "on",
    deps = [":purelib"],
)

go_binary(
    name = "c_shared_bin",
    srcs = ["pure_main.go"],
    linkmode = "c-shared",
    deps = [":purelib"],
)

go_binary(
    name = "c_archive_bin",
    srcs = ["pure_main.go"],
    linkmode = "c-archive",
    deps = [":purelib"],
)

go_binary(
    name = "plugin_bin",
    srcs = ["pure_main.go"],
    linkmode = "plugin",
    deps = [":purelib"],
)

go_binary(
    name = "extldflags_bin",
    srcs = ["pure_main.go"],
    gc_linkopts = ["-extldflags", "-Wl,-rpath,/tmp"],
    deps = [":purelib"],
)

go_binary(
    name = "msan_flag_bin",
    srcs = ["pure_main.go"],
    gc_linkopts = ["-msan"],
    deps = [":purelib"],
)

go_binary(
    name = "buildmode_flag_bin",
    srcs = ["pure_main.go"],
    gc_linkopts = ["-buildmode=exe"],
    deps = [":purelib"],
)

go_binary(
    name = "gotags_bin",
    srcs = ["pure_main.go"],
    gotags = [
        "netgo",
        "osusergo",
    ],
    deps = [":purelib"],
)

go_binary(
    name = "syso_bin",
    srcs = [
        "empty.syso",
        "pure_main.go",
    ],
    deps = [":purelib"],
)

go_binary(
    name = "pure_off_bin",
    srcs = ["pure_main.go"],
    pure = "off",
    deps = [":purelib"],
)

go_binary(
    name = "cgo_bin",
    srcs = ["cgo_main.go"],
    deps = [":cgolib"],
)

go_binary(
    name = "transitive_cgo_bin",
    srcs = ["transitive_cgo_main.go"],
    deps = [":midlib"],
)

go_binary(
    name = "buildinfo_bin",
    srcs = ["buildinfo_main.go"],
)

# Go assembly and its header do not use cgo.
go_library(
    name = "asmlib",
    srcs = [
        "asm_lib.go",
        "asm_lib.h",
        "asm_lib.s",
    ],
    importpath = "example.com/asmlib",
)

go_binary(
    name = "asm_bin",
    srcs = ["asm_main.go"],
    deps = [":asmlib"],
)

# Uses cgo only on Windows.
go_library(
    name = "selectcgo",
    srcs = [
        "select_cgo_windows.go",
        "select_pure.go",
    ],
    cgo = select({
        "@io_bazel_rules_go//go/platform:windows": True,
        "//conditions:default": False,
    }),
    importpath = "example.com/selectcgo",
)

go_binary(
    name = "select_cgo_bin",
    srcs = ["select_main.go"],
    deps = [":selectcgo"],
)

go_library(
    name = "deponlibandcgo",
    srcs = ["dep_on_lib_and_cgo.go"],
    importpath = "example.com/deponlibandcgo",
    deps = [
        ":cgolib",
        ":purelib",
    ],
)

# The external test imports deponlibandcgo, which depends on the embedded
# purelib. So the test recompiles deponlibandcgo and takes cgolib from the
# stub GoArchive branch of _recompile_external_deps.
go_test(
    name = "stub_test",
    srcs = ["stub_test.go"],
    embed = [":purelib"],
    deps = [":deponlibandcgo"],
)

cc_toolchain_files(
    name = "cc_files",
)
`,
	})
}

// netgoFlags makes the standard library free of cgo on every platform.
var netgoFlags = []string{"--define=gotags=netgo,osusergo"}

// internalLinkFlags also sets the flag that removes the C/C++ toolchain from
// the link step of binaries that can link internally.
var internalLinkFlags = []string{"--define=gotags=netgo,osusergo", "--@io_bazel_rules_go//go/config:internal_link"}

// goVars holds the environment variables that rules_go sets for GoLink
// without a C/C++ toolchain.
var goVars = map[string]bool{
	"CGO_ENABLED":  true,
	"GOAMD64":      true,
	"GOARCH":       true,
	"GOARM":        true,
	"GODEBUG":      true,
	"GOEXPERIMENT": true,
	"GOOS":         true,
	"GOPATH":       true,
	"GOROOT":       true,
	"GOROOT_FINAL": true,
	"GOTOOLCHAIN":  true,
}

// The -linkmode that GoLink passes to the Go linker. auto means no -linkmode
// flag, so the Go linker chooses.
const (
	auto     = "auto"
	internal = "internal"
	external = "external"
)

type linkModeCase struct {
	target string
	want   string
}

// TestDefaultLinkerChoice checks the link mode without the internal_link
// flag. rules_go lets the Go linker choose for a binary that can link
// internally, and keeps its own rules for the other binaries.
func TestDefaultLinkerChoice(t *testing.T) {
	// The default link mode is pie on macOS and Windows, and rules_go forces
	// the external linker for pie when the Go linker cannot choose.
	ownRules := auto
	if defaultPIE() {
		ownRules = external
	}
	pie := external
	if internalPIE() {
		pie = auto
	}
	// On macOS, rules_go passes no -extld for c-archive (extld_from_cc_toolchain
	// in go/private/mode.bzl), so it does not force the external linker. The Go
	// linker always links c-archive externally.
	cArchive := external
	if runtime.GOOS == "darwin" {
		cArchive = auto
	}
	cases := []linkModeCase{
		{"//src:pure_bin", auto},
		{"//src:pure_test", auto},
		{"//src:static_bin", auto},
		{"//src:static_normal_bin", auto},
		{"//src:asm_bin", auto},
		{"//src:buildinfo_bin", auto},
		{"//src:plugin_host_bin", auto},
		{"//src:pie_pure_bin", pie},
		{"//src:static_pie_bin", pie},
		{"//src:static_cgo_bin", external},
		{"//src:static_pure_off_bin", external},
		{"//src:race_bin", external},
		{"//src:c_shared_bin", external},
		{"//src:c_archive_bin", cArchive},
		{"//src:cgo_bin", ownRules},
		{"//src:transitive_cgo_bin", ownRules},
		{"//src:syso_bin", ownRules},
		{"//src:pure_off_bin", ownRules},
		{"//src:stub_test", ownRules},
		{"//src:extldflags_bin", ownRules},
		{"//src:msan_flag_bin", ownRules},
		{"//src:buildmode_flag_bin", ownRules},
		{"//src:msan_bin", ownRules},
	}
	if runtime.GOOS == "windows" {
		cases = append(cases, linkModeCase{"//src:select_cgo_bin", external})
	} else {
		cases = append(cases, linkModeCase{"//src:select_cgo_bin", auto})
	}
	if runtime.GOOS == "linux" || runtime.GOOS == "darwin" {
		// Go supports plugins only on these hosts.
		cases = append(cases, linkModeCase{"//src:plugin_bin", external})
	}
	actions := checkLinkModes(t, netgoFlags, cases)

	// The Go linker can still select the external linker, so the link step
	// keeps the C/C++ toolchain.
	ccFiles := strings.Fields(string(readOutput(t, netgoFlags, "//src:cc_files")))
	for _, target := range []string{"//src:pure_bin", "//src:static_bin", "//src:pie_pure_bin"} {
		action := actions[target]
		if !slices.Contains(action.args, "-extld") {
			t.Errorf("%s: want -extld, got args: %v", target, action.args)
		}
		if _, ok := action.env["PATH"]; !ok {
			t.Errorf("%s: GoLink environment has no PATH from the C/C++ toolchain: %v", target, action.env)
		}
		for _, f := range ccFiles {
			if !action.inputs[f] {
				t.Errorf("%s: GoLink does not have the C/C++ toolchain file %s in its inputs", target, f)
			}
		}
	}
}

// TestDefaultOptOuts checks that the opt-outs give the rules of rules_go,
// which force the external linker for a static binary.
func TestDefaultOptOuts(t *testing.T) {
	for _, flag := range []string{
		"--@io_bazel_rules_go//go/config:gc_linkopts=-linkmode=external",
		"--linkopt=-Wl,-v",
	} {
		checkLinkModes(t, append(slices.Clip(netgoFlags), flag), []linkModeCase{{"//src:static_bin", external}})
	}
}

// TestInternalLinkFlag checks the link mode with the internal_link flag.
func TestInternalLinkFlag(t *testing.T) {
	pie := external
	if internalPIE() {
		pie = internal
	}
	cases := []linkModeCase{
		{"//src:pure_bin", internal},
		{"//src:pure_test", internal},
		{"//src:static_bin", internal},
		{"//src:static_normal_bin", internal},
		{"//src:asm_bin", internal},
		{"//src:buildinfo_bin", internal},
		{"//src:pie_pure_bin", pie},
		{"//src:static_pie_bin", pie},
	}
	if runtime.GOOS == "windows" {
		cases = append(cases, linkModeCase{"//src:select_cgo_bin", external})
	} else {
		cases = append(cases, linkModeCase{"//src:select_cgo_bin", internal})
	}
	actions := checkLinkModes(t, internalLinkFlags, cases)

	// These binaries can need the external linker, so they must not link
	// internally. The plugin host sets pure = "off", as the docs require.
	notInternal := []string{
		"//src:plugin_host_off_bin",
		"//src:static_cgo_bin",
		"//src:static_pure_off_bin",
		"//src:extldflags_bin",
		"//src:msan_flag_bin",
		"//src:buildmode_flag_bin",
		"//src:cgo_bin",
		"//src:transitive_cgo_bin",
		"//src:syso_bin",
		"//src:pure_off_bin",
		"//src:stub_test",
		"//src:race_bin",
		"//src:msan_bin",
		"//src:c_shared_bin",
		"//src:c_archive_bin",
	}
	if runtime.GOOS == "linux" || runtime.GOOS == "darwin" {
		notInternal = append(notInternal, "//src:plugin_bin")
	}
	notInternalActions := goLinkActions(t, internalLinkFlags, notInternal...)
	for _, target := range notInternal {
		if got := linkMode(notInternalActions[target].args); got == internal {
			t.Errorf("%s: got -linkmode internal, want the Go linker to keep the external linker; args: %v", target, notInternalActions[target].args)
		}
	}

	for _, target := range []string{"//src:pure_bin", "//src:pure_test"} {
		args := actions[target].args
		if slices.Contains(args, "-extld") || slices.Contains(args, "-extldflags") || slices.Contains(args, "-extar") {
			t.Errorf("%s: want no -extld, -extldflags, or -extar, got args: %v", target, args)
		}
	}
	if pieArgs := actions["//src:pie_pure_bin"].args; !hasFlagPair(pieArgs, "-buildmode", "pie") {
		t.Errorf("//src:pie_pure_bin: want -buildmode pie, got args: %v", pieArgs)
	}
	foundRpath := false
	for _, a := range notInternalActions["//src:extldflags_bin"].args {
		if strings.Contains(a, "-Wl,-rpath,/tmp") {
			foundRpath = true
			break
		}
	}
	if !foundRpath {
		t.Errorf("//src:extldflags_bin: want the -extldflags payload, got args: %v", notInternalActions["//src:extldflags_bin"].args)
	}
}

func TestInternalLinkFlagOptOuts(t *testing.T) {
	for _, flag := range []string{
		"--@io_bazel_rules_go//go/config:gc_linkopts=-linkmode=external",
		"--linkopt=-Wl,-v",
	} {
		actions := goLinkActions(t, append(slices.Clip(internalLinkFlags), flag), "//src:pure_bin")
		if got := linkMode(actions["//src:pure_bin"].args); got == internal {
			t.Errorf("with %s: want no -linkmode internal, got args: %v", flag, actions["//src:pure_bin"].args)
		}
	}
}

func TestStandardLibraryTags(t *testing.T) {
	const flag = "--@io_bazel_rules_go//go/config:internal_link"

	// A tag that the standard library ignores changes nothing.
	checkLinkModes(t, []string{flag, "--define=gotags=netgo,osusergo,customtag"}, []linkModeCase{{"//src:pure_bin", internal}})

	// Tags set on the binary reach the standard library (#3488).
	checkLinkModes(t, []string{flag}, []linkModeCase{{"//src:gotags_bin", internal}})

	// Without netgo and osusergo, net and os/user use cgo on Linux, but not on
	// macOS or Windows.
	var want string
	switch runtime.GOOS {
	case "linux":
		want = auto
	case "darwin", "windows":
		want = internal
	default:
		t.Logf("no expectation for %s without netgo and osusergo", runtime.GOOS)
		return
	}
	checkLinkModes(t, []string{flag, "--define=gotags=customtag"}, []linkModeCase{{"//src:pure_bin", want}})
}

func TestInternalLinkInputs(t *testing.T) {
	// cc_files lists cc_toolchain.all_files of the C/C++ toolchain that Bazel
	// resolves for the host. This works for any toolchain. File names or input
	// counts would depend on the toolchain.
	ccFiles := strings.Fields(string(readOutput(t, internalLinkFlags, "//src:cc_files")))
	actions := goLinkActions(t, internalLinkFlags, "//src:pure_bin", "//src:pure_test", "//src:pure_off_bin")
	externalAction := actions["//src:pure_off_bin"]
	if len(ccFiles) == 0 {
		t.Log("the C/C++ toolchain has no files")
	}
	for _, f := range ccFiles {
		if !externalAction.inputs[f] {
			t.Errorf("external GoLink does not have the C/C++ toolchain file %s in its inputs", f)
		}
	}
	if _, ok := externalAction.env["PATH"]; !ok {
		t.Errorf("external GoLink environment has no PATH from the C/C++ toolchain: %v", externalAction.env)
	}

	for _, target := range []string{"//src:pure_bin", "//src:pure_test"} {
		internalAction := actions[target]
		for _, f := range ccFiles {
			if internalAction.inputs[f] {
				t.Errorf("%s: GoLink has the C/C++ toolchain file %s in its inputs", target, f)
			}
		}
		for k, v := range internalAction.env {
			if !goVars[k] {
				t.Errorf("%s: GoLink environment has %s=%q from the C/C++ toolchain", target, k, v)
			}
		}
		if internalAction.env["CGO_ENABLED"] != "1" {
			t.Errorf("%s: GoLink environment has CGO_ENABLED=%q, want 1", target, internalAction.env["CGO_ENABLED"])
		}
	}
}

// TestExecTools checks that the internal_link flag also reaches Go tools in
// the exec configuration, because the flag has universal scope.
func TestExecTools(t *testing.T) {
	ccFiles := strings.Fields(string(readOutput(t, netgoFlags, "//src:cc_files")))
	cases := []struct {
		flags []string
		want  string
	}{
		{netgoFlags, auto},
		{internalLinkFlags, internal},
	}

	// Bazel 10 does not pass Starlark flags to the exec configuration by
	// default. The universal scope of internal_link opts out of that. The
	// build tags come from --define, which Bazel passes in all versions.
	const excludeFlag = "--incompatible_exclude_starlark_flags_from_exec_config"
	if err := bazel_testing.RunBazel("build", "//src:tool_gen", "--nobuild", excludeFlag); err != nil {
		if !strings.Contains(err.Error(), "Unrecognized option: "+excludeFlag) {
			t.Fatalf("bazel build //src:tool_gen %s: %v", excludeFlag, err)
		}
		t.Logf("Bazel does not support %s", excludeFlag)
	} else {
		cases = append(cases, struct {
			flags []string
			want  string
		}{append(slices.Clip(internalLinkFlags), excludeFlag), internal})
	}

	const query = `mnemonic("GoLink", deps(//src:tool_gen))`
	for _, c := range cases {
		actions := goLinkActionsForQuery(t, c.flags, query)
		tool, ok := actions["//src:tool_bin"]
		if !ok {
			t.Fatalf("with %q: no GoLink action for //src:tool_bin in %v", c.flags, actions)
		}
		if !tool.isTool {
			t.Errorf("with %q: the GoLink action for //src:tool_bin is not in the exec configuration", c.flags)
		}
		if got := linkMode(tool.args); got != c.want {
			t.Errorf("with %q: //src:tool_bin got -linkmode %s, want %s; args: %v", c.flags, got, c.want, tool.args)
		}
		for _, f := range ccFiles {
			if c.want == internal && tool.inputs[f] {
				t.Errorf("with %q: //src:tool_bin GoLink has the C/C++ toolchain file %s in its inputs", c.flags, f)
			}
			if c.want != internal && !tool.inputs[f] {
				t.Errorf("with %q: //src:tool_bin GoLink does not have the C/C++ toolchain file %s in its inputs", c.flags, f)
			}
		}
	}
}

func TestDefaultBinaries(t *testing.T) {
	targets := []string{
		"//src:pure_bin",
		"//src:static_bin",
		"//src:asm_bin",
		"//src:pure_off_bin",
		"//src:cgo_bin",
		"//src:transitive_cgo_bin",
	}
	if internalPIE() {
		targets = append(targets, "//src:pie_pure_bin")
	}
	if err := bazel_testing.RunBazel(append(append([]string{"build"}, netgoFlags...), targets...)...); err != nil {
		t.Fatalf("bazel build: %v", err)
	}
	if err := bazel_testing.RunBazel(append(append([]string{"test"}, netgoFlags...), "//src:pure_test")...); err != nil {
		t.Fatalf("bazel test //src:pure_test: %v", err)
	}

	if runtime.GOOS != "linux" {
		t.Logf("the ELF check runs only on Linux, not on %s", runtime.GOOS)
		return
	}

	// The Go linker links the static binary internally, so it has no
	// interpreter.
	checkNoInterpreter(t, openELF(t, netgoFlags, "//src:static_bin"), "//src:static_bin")
}

func TestInternalLinkBinaries(t *testing.T) {
	targets := []string{
		"//src:pure_bin",
		"//src:static_bin",
		"//src:asm_bin",
	}
	if err := bazel_testing.RunBazel(append(append([]string{"build"}, internalLinkFlags...), targets...)...); err != nil {
		t.Fatalf("bazel build: %v", err)
	}
	if err := bazel_testing.RunBazel(append(append([]string{"test"}, internalLinkFlags...), "//src:pure_test")...); err != nil {
		t.Fatalf("bazel test //src:pure_test: %v", err)
	}

	// The build info records CGO_ENABLED=1, as go build does, because the
	// packages were compiled with cgo enabled.
	out, err := bazel_testing.BazelOutput(append(append([]string{"run"}, internalLinkFlags...), "//src:buildinfo_bin")...)
	if err != nil {
		t.Fatalf("bazel run //src:buildinfo_bin: %v", err)
	}
	var settings map[string]string
	if err := json.Unmarshal(out, &settings); err != nil {
		t.Fatalf("unmarshal build settings %q: %v", out, err)
	}
	if settings["CGO_ENABLED"] != "1" {
		t.Errorf("//src:buildinfo_bin: build info has CGO_ENABLED=%q, want 1", settings["CGO_ENABLED"])
	}
}

func TestInternalLinkELF(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skipf("the ELF checks run only on Linux, not on %s", runtime.GOOS)
	}

	// In normal mode, an internal link without cgo gives a static binary.
	checkNoInterpreter(t, openELF(t, internalLinkFlags, "//src:static_bin"), "//src:static_bin")

	// From Go 1.24, internal links write a GNU build ID.
	pure := openELF(t, internalLinkFlags, "//src:pure_bin")
	note := pure.Section(".note.gnu.build-id")
	if note == nil {
		t.Fatal("//src:pure_bin has no .note.gnu.build-id section")
	}
	data, err := note.Data()
	if err != nil {
		t.Fatal(err)
	}
	if len(data) <= 16 || strings.Trim(string(data[16:]), "\x00") == "" {
		t.Errorf("//src:pure_bin has an empty GNU build ID note: %x", data)
	}
}

// defaultPIE reports whether rules_go builds PIE binaries for the host by
// default.
func defaultPIE() bool {
	switch runtime.GOOS {
	case "android", "darwin", "ios", "windows":
		return true
	}
	return false
}

// internalPIE reports whether the Go linker can link PIE binaries for the
// host internally, without a C/C++ toolchain.
func internalPIE() bool {
	switch runtime.GOOS + "/" + runtime.GOARCH {
	case "darwin/amd64", "darwin/arm64", "windows/386", "windows/amd64", "windows/arm64":
		return true
	}
	return false
}

// linkMode returns the -linkmode that GoLink passes, or auto if it passes
// none.
func linkMode(args []string) string {
	switch {
	case hasFlagPair(args, "-linkmode", internal):
		return internal
	case hasFlagPair(args, "-linkmode", external):
		return external
	default:
		return auto
	}
}

// checkLinkModes checks the -linkmode of each case and returns the GoLink
// actions.
func checkLinkModes(t *testing.T, flags []string, cases []linkModeCase) map[string]goLinkAction {
	t.Helper()
	var targets []string
	for _, c := range cases {
		targets = append(targets, c.target)
	}
	actions := goLinkActions(t, flags, targets...)
	for _, c := range cases {
		if got := linkMode(actions[c.target].args); got != c.want {
			t.Errorf("with %q, %s: got -linkmode %s, want %s; args: %v", flags, c.target, got, c.want, actions[c.target].args)
		}
	}
	return actions
}

func checkNoInterpreter(t *testing.T, f *elf.File, target string) {
	t.Helper()
	for _, prog := range f.Progs {
		if prog.Type == elf.PT_INTERP {
			t.Errorf("%s has a PT_INTERP program header, so it is not static", target)
		}
	}
}

func hasFlagPair(args []string, flag, val string) bool {
	for i := 0; i+1 < len(args); i++ {
		if args[i] == flag && args[i+1] == val {
			return true
		}
	}
	return slices.Contains(args, flag+"="+val)
}

type goLinkAction struct {
	args   []string
	env    map[string]string
	inputs map[string]bool
	// isTool is true for an action in the exec configuration.
	isTool bool
}

// goLinkActions returns the GoLink action of each target, keyed by label.
func goLinkActions(t *testing.T, flags []string, targets ...string) map[string]goLinkAction {
	t.Helper()
	actions := goLinkActionsForQuery(t, flags, `mnemonic("GoLink", `+strings.Join(targets, " + ")+`)`)
	for _, target := range targets {
		if _, ok := actions[target]; !ok {
			t.Fatalf("no GoLink action for %s in the aquery output", target)
		}
	}
	return actions
}

// goLinkActionsForQuery returns the actions that an aquery expression
// selects, keyed by label. It fails if a label has more than one action.
func goLinkActionsForQuery(t *testing.T, flags []string, query string) map[string]goLinkAction {
	t.Helper()
	args := append([]string{"aquery", "--output=jsonproto"}, flags...)
	args = append(args, query)
	out, err := bazel_testing.BazelOutput(args...)
	if err != nil {
		t.Fatalf("bazel %s: %v", strings.Join(args, " "), err)
	}
	var res struct {
		Artifacts []struct {
			ID             int `json:"id"`
			PathFragmentID int `json:"pathFragmentId"`
		} `json:"artifacts"`
		Actions []struct {
			TargetID             int      `json:"targetId"`
			ConfigurationID      int      `json:"configurationId"`
			Arguments            []string `json:"arguments"`
			EnvironmentVariables []struct {
				Key   string `json:"key"`
				Value string `json:"value"`
			} `json:"environmentVariables"`
			InputDepSetIDs []int `json:"inputDepSetIds"`
		} `json:"actions"`
		Targets []struct {
			ID    int    `json:"id"`
			Label string `json:"label"`
		} `json:"targets"`
		Configuration []struct {
			ID     int  `json:"id"`
			IsTool bool `json:"isTool"`
		} `json:"configuration"`
		DepSetOfFiles []struct {
			ID                  int   `json:"id"`
			DirectArtifactIDs   []int `json:"directArtifactIds"`
			TransitiveDepSetIDs []int `json:"transitiveDepSetIds"`
		} `json:"depSetOfFiles"`
		PathFragments []struct {
			ID       int    `json:"id"`
			Label    string `json:"label"`
			ParentID int    `json:"parentId"`
		} `json:"pathFragments"`
	}
	if err := json.Unmarshal(out, &res); err != nil {
		t.Fatalf("unmarshal aquery output: %v", err)
	}

	type fragment struct {
		label  string
		parent int
	}
	fragments := make(map[int]fragment)
	for _, f := range res.PathFragments {
		fragments[f.ID] = fragment{f.Label, f.ParentID}
	}
	pathOf := func(id int) string {
		var parts []string
		for ; id != 0; id = fragments[id].parent {
			parts = append(parts, fragments[id].label)
		}
		slices.Reverse(parts)
		return strings.Join(parts, "/")
	}
	artifacts := make(map[int]string)
	for _, a := range res.Artifacts {
		artifacts[a.ID] = pathOf(a.PathFragmentID)
	}
	type depSet struct {
		direct, transitive []int
	}
	depSets := make(map[int]depSet)
	for _, d := range res.DepSetOfFiles {
		depSets[d.ID] = depSet{d.DirectArtifactIDs, d.TransitiveDepSetIDs}
	}
	labels := make(map[int]string)
	for _, target := range res.Targets {
		label := target.Label
		if i := strings.Index(label, "//"); i >= 0 {
			label = label[i:]
		}
		labels[target.ID] = label
	}
	toolConfigurations := make(map[int]bool)
	for _, c := range res.Configuration {
		toolConfigurations[c.ID] = c.IsTool
	}

	actions := make(map[string]goLinkAction)
	for _, a := range res.Actions {
		label := labels[a.TargetID]
		if _, ok := actions[label]; ok {
			t.Fatalf("more than one GoLink action for %s", label)
		}
		env := make(map[string]string)
		for _, kv := range a.EnvironmentVariables {
			env[kv.Key] = kv.Value
		}
		inputs := make(map[string]bool)
		visited := make(map[int]bool)
		stack := slices.Clone(a.InputDepSetIDs)
		for len(stack) > 0 {
			id := stack[len(stack)-1]
			stack = stack[:len(stack)-1]
			if visited[id] {
				continue
			}
			visited[id] = true
			for _, artifact := range depSets[id].direct {
				inputs[artifacts[artifact]] = true
			}
			stack = append(stack, depSets[id].transitive...)
		}
		actions[label] = goLinkAction{args: a.Arguments, env: env, inputs: inputs, isTool: toolConfigurations[a.ConfigurationID]}
	}
	return actions
}

// readOutput builds target with flags and returns the content of its single
// output file.
func readOutput(t *testing.T, flags []string, target string) []byte {
	t.Helper()
	data, err := os.ReadFile(outputPath(t, flags, target))
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func openELF(t *testing.T, flags []string, target string) *elf.File {
	t.Helper()
	f, err := elf.Open(outputPath(t, flags, target))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { f.Close() })
	return f
}

// outputPath builds target with flags and returns the path of its output
// file.
func outputPath(t *testing.T, flags []string, target string) string {
	t.Helper()
	if err := bazel_testing.RunBazel(append(append([]string{"build"}, flags...), target)...); err != nil {
		t.Fatalf("bazel build %s: %v", target, err)
	}
	out, err := bazel_testing.BazelOutput(append(append([]string{"cquery", "--output=files"}, flags...), target)...)
	if err != nil {
		t.Fatalf("bazel cquery %s: %v", target, err)
	}
	return strings.TrimSpace(string(out))
}
