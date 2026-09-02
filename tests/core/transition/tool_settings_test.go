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

package tool_settings_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"testing"

	"github.com/bazelbuild/rules_go/go/tools/bazel_testing"
)

func TestMain(m *testing.M) {
	bazel_testing.TestMain(m, bazel_testing.Args{
		Main: `
-- BUILD.bazel --
load("@io_bazel_rules_go//go:def.bzl", "go_binary", "go_library", "nogo")

nogo(
    name = "my_nogo",
    vet = True,
    visibility = ["//visibility:public"],
)

go_library(
    name = "lib",
    srcs = ["lib.go"],
    importpath = "example.com/lib",
)

go_binary(
    name = "plain",
    srcs = ["main.go"],
    deps = [":lib"],
)

go_binary(
    name = "static_attr",
    srcs = ["main.go"],
    static = "on",
    deps = [":lib"],
)

go_binary(
    name = "pure_attr",
    srcs = ["main.go"],
    pure = "on",
    deps = [":lib"],
)

genrule(
    name = "tool_user",
    outs = ["tool_user.txt"],
    cmd = "$(location :plain) > $@",
    tools = [":plain"],
)
-- main.go --
package main

func main() {}
-- lib.go --
package lib
`,
		ModuleFileSuffix: `
go_sdk = use_extension("@io_bazel_rules_go//go:extensions.bzl", "go_sdk")
go_sdk.nogo(nogo = "//:my_nogo")
`,
	})
}

// TestCommandLineSettingsReachTools verifies that a value set on the command
// line applies to Go tool binaries. Tools have to run on the execution
// platform, so one that is linked dynamically against a libc that isn't
// installed there breaks the build. See #4378.
func TestCommandLineSettingsReachTools(t *testing.T) {
	for _, setting := range []string{"static", "pure"} {
		t.Run(setting, func(t *testing.T) {
			got := nogoGoOptions(t, "//:plain", "--@io_bazel_rules_go//go/config:"+setting)
			want := setting + "=true"
			if !contains(got, want) {
				t.Errorf("nogo is not built with %s: got %s", want, strings.Join(got, ", "))
			}
		})
	}
}

// TestCommandLineSettingsReachToolsWithExcludedStarlarkFlags verifies the same
// for Bazel's upcoming default of not propagating Starlark flags into the exec
// configuration, which the settings opt out of via scope = "universal".
func TestCommandLineSettingsReachToolsWithExcludedStarlarkFlags(t *testing.T) {
	const excludeFlag = "--incompatible_exclude_starlark_flags_from_exec_config"
	if err := bazel_testing.RunBazel("build", "//:plain", "--nobuild", excludeFlag); err != nil {
		// The flag only exists on Bazel 9+. On older versions, skip rather than
		// fail so the test stays green across the supported Bazel matrix.
		if strings.Contains(err.Error(), "Unrecognized option: "+excludeFlag) {
			t.Skipf("Bazel does not support %s; skipping", excludeFlag)
		}
		t.Fatalf("bazel build //:plain %s: %v", excludeFlag, err)
	}

	for _, setting := range []string{"static", "pure"} {
		t.Run(setting, func(t *testing.T) {
			got := nogoGoOptions(t, "//:plain", excludeFlag, "--@io_bazel_rules_go//go/config:"+setting)
			if want := setting + "=true"; !contains(got, want) {
				t.Errorf("nogo is not built with %s: got %s", want, strings.Join(got, ", "))
			}
		})
		t.Run(setting+"_attr", func(t *testing.T) {
			got := nogoGoOptions(t, "//:"+setting+"_attr", excludeFlag)
			if contains(got, setting+"=true") {
				t.Errorf("nogo inherited %s from the rule attribute: got %s", setting, strings.Join(got, ", "))
			}
		})
	}
}

// TestStaticAttributeDoesNotReachDependencies verifies that the static
// attribute only affects the link action of the binary it is set on. Static
// linking is a property of that single link, so applying it through the
// configuration would recompile every dependency for no reason.
func TestStaticAttributeDoesNotReachDependencies(t *testing.T) {
	if err := bazel_testing.RunBazel("build", "//:plain", "//:static_attr", "--nobuild"); err != nil {
		t.Fatalf("bazel build //:plain //:static_attr: %v", err)
	}
	hashes := make(map[string][]string)
	for _, target := range []string{"//:plain", "//:static_attr"} {
		query := fmt.Sprintf("deps(%s) intersect //:lib", target)
		out, err := bazel_testing.BazelOutput("cquery", "--output=jsonproto", query)
		if err != nil {
			t.Fatalf("bazel cquery '%s': %v", query, err)
		}
		hashes[target] = extractConfigHashes(t, bytes.TrimSpace(out))
		if len(hashes[target]) != 1 {
			t.Fatalf("expected %s to depend on //:lib in exactly one configuration, got %d", target, len(hashes[target]))
		}
	}
	plain, static := hashes["//:plain"][0], hashes["//:static_attr"][0]
	if plain != static {
		t.Errorf("//:lib is built in different configurations for //:plain and //:static_attr, differing in: %s",
			strings.Join(getGoOptions(t, plain, static), ", "))
	}
}

// TestRuleAttributesDoNotReachTools verifies that the static and pure
// attributes of an individual rule do not propagate to Go tool binaries, which
// would build one copy of every tool per distinct attribute value.
func TestRuleAttributesDoNotReachTools(t *testing.T) {
	for _, setting := range []string{"static", "pure"} {
		t.Run(setting, func(t *testing.T) {
			got := nogoGoOptions(t, "//:"+setting+"_attr")
			if contains(got, setting+"=true") {
				t.Errorf("nogo inherited %s from the rule attribute: got %s", setting, strings.Join(got, ", "))
			}
		})
	}
}

// TestToolsShareStdlib verifies that all Go binaries built for the exec
// configuration, whether as plain tools of a genrule or through
// go_tool_transition like nogo, are built against a single standard library.
// A tool transition that sets a setting to anything but its default value
// gives the tools it applies to a configuration of their own and thus a
// second copy of the standard library and of every dependency they share
// with other tools.
func TestToolsShareStdlib(t *testing.T) {
	// See nogoGoOptions for why the build is necessary.
	if err := bazel_testing.RunBazel("build", "//:tool_user", "--nobuild"); err != nil {
		t.Fatalf("bazel build //:tool_user: %v", err)
	}
	query := "deps(//:tool_user) intersect @io_bazel_rules_go//:stdlib"
	out, err := bazel_testing.BazelOutput("cquery", "--output=jsonproto", query)
	if err != nil {
		t.Fatalf("bazel cquery '%s': %v", query, err)
	}
	hashes := extractConfigHashes(t, bytes.TrimSpace(out))
	if len(hashes) == 1 {
		return
	}
	var diffs []string
	for i := 1; i < len(hashes); i++ {
		diffs = append(diffs, strings.Join(getGoOptions(t, hashes[0], hashes[i]), ", "))
	}
	t.Errorf("the tools of //:tool_user are built against %d standard libraries in configurations differing in: %s",
		len(hashes), strings.Join(diffs, "; "))
}

// nogoGoOptions returns the rules_go settings that the nogo binary reachable
// from target is configured with and that differ from their default value.
func nogoGoOptions(t *testing.T, target string, flags ...string) []string {
	// Analyze the targets to ensure that MODULE.bazel.lock has been created,
	// otherwise bazel config will fail after the cquery command due to the
	// Skyframe invalidation caused by a changed file.
	if err := bazel_testing.RunBazel(append([]string{"build", target, "--nobuild"}, flags...)...); err != nil {
		t.Fatalf("bazel build %s: %v", target, err)
	}

	query := fmt.Sprintf("deps(%s) intersect //:my_nogo_actual", target)
	out, err := bazel_testing.BazelOutput(append(
		[]string{"cquery", "--output=jsonproto", query},
		flags...,
	)...)
	if err != nil {
		t.Fatalf("bazel cquery '%s': %v", query, err)
	}
	hashes := extractConfigHashes(t, bytes.TrimSpace(out))
	if len(hashes) == 0 {
		t.Fatalf("%s does not depend on //:my_nogo_actual", target)
	}
	if len(hashes) > 1 {
		// bazel config only reports the options the configs differ in.
		if diff := getGoOptions(t, hashes...); len(diff) != 0 {
			t.Fatalf("%s depends on //:my_nogo_actual in configs differing in: %s",
				target, strings.Join(diff, ", "))
		}
	}
	return getGoOptions(t, hashes[0])
}

func contains(haystack []string, needle string) bool {
	for _, s := range haystack {
		if s == needle {
			return true
		}
	}
	return false
}

func extractConfigHashes(t *testing.T, rawJSONOut []byte) []string {
	var jsonOut struct {
		Results []struct {
			Configuration struct {
				Checksum string `json:"checksum"`
			} `json:"configuration"`
		} `json:"results"`
	}
	if err := json.Unmarshal(rawJSONOut, &jsonOut); err != nil {
		t.Fatalf("Failed to decode bazel cquery JSON output %v: %q", err, string(rawJSONOut))
	}
	var hashes []string
	for _, result := range jsonOut.Results {
		hashes = append(hashes, result.Configuration.Checksum)
	}
	return hashes
}

func getGoOptions(t *testing.T, hashes ...string) []string {
	out, err := bazel_testing.BazelOutput(append([]string{"config", "--output=json"}, hashes...)...)
	if err != nil {
		t.Fatalf("bazel config %s: %v", strings.Join(hashes, " "), err)
	}
	var jsonOut struct {
		// Set when a single configuration is requested.
		Fragments []struct {
			Name    string            `json:"name"`
			Options map[string]string `json:"options"`
		} `json:"fragmentOptions"`
		// Set when two configurations are diffed.
		FragmentsDiff []struct {
			Name        string `json:"name"`
			OptionsDiff map[string]struct {
				First  string `json:"first"`
				Second string `json:"second"`
			} `json:"optionsDiff"`
		} `json:"fragmentsDiff"`
	}
	if err := json.Unmarshal(bytes.TrimSpace(out), &jsonOut); err != nil {
		t.Fatalf("Failed to decode bazel config JSON output %v: %q", err, string(out))
	}
	var goOptions []string
	for _, fragment := range jsonOut.Fragments {
		if fragment.Name != "user-defined" {
			continue
		}
		for key, value := range fragment.Options {
			if name, ok := goOptionName(key); ok {
				goOptions = append(goOptions, fmt.Sprintf("%s=%s", name, value))
			}
		}
	}
	for _, fragment := range jsonOut.FragmentsDiff {
		if fragment.Name != "user-defined" {
			continue
		}
		for key, diff := range fragment.OptionsDiff {
			if name, ok := goOptionName(key); ok {
				goOptions = append(goOptions, fmt.Sprintf("%s=%s vs %s", name, diff.First, diff.Second))
			}
		}
	}
	sort.Strings(goOptions)
	return goOptions
}

// goOptionName returns the part of a rules_go setting's label that identifies
// it. The repository part depends on the canonical repository name and is
// dropped. Settings in //go/private are reported with a "private:" prefix.
func goOptionName(key string) (string, bool) {
	if _, name, found := strings.Cut(key, "//go/config:"); found {
		return name, true
	}
	if _, name, found := strings.Cut(key, "//go/private:"); found {
		return "private:" + name, true
	}
	return "", false
}

// TestStdlibSharesConfigurationWithTarget verifies that the standard library
// a Go binary links against is built in the binary's own configuration when
// nothing but default settings are in effect.
func TestStdlibSharesConfigurationWithTarget(t *testing.T) {
	// Restrict to the target configuration: the server may also hold //:plain
	// in the exec configuration from an earlier test.
	targetHashes := configHashes(t, "config(//:plain, target)")
	if len(targetHashes) != 1 {
		t.Fatalf("expected //:plain to be built in exactly one configuration, got %d", len(targetHashes))
	}
	stdlibHashes := configHashes(t, "deps(//:plain) intersect @io_bazel_rules_go//:stdlib")
	if contains(stdlibHashes, targetHashes[0]) {
		return
	}
	var diffs []string
	for _, hash := range stdlibHashes {
		diffs = append(diffs, strings.Join(getGoOptions(t, targetHashes[0], hash), ", "))
	}
	t.Errorf("no stdlib is built in the configuration of //:plain, the stdlib configurations differ from it in: %s",
		strings.Join(diffs, "; "))
}

// TestStdlibSharesConfigurationWithExecTools verifies the same for the exec
// configuration: every standard library reachable from a genrule tool,
// including the one nogo is built against, is built in the configuration of
// the tool itself.
func TestStdlibSharesConfigurationWithExecTools(t *testing.T) {
	toolHashes := configHashes(t, "deps(//:tool_user) intersect //:plain")
	if len(toolHashes) != 1 {
		t.Fatalf("expected //:tool_user to depend on //:plain in exactly one configuration, got %d", len(toolHashes))
	}
	stdlibHashes := configHashes(t, "deps(//:tool_user) intersect @io_bazel_rules_go//:stdlib")
	var diffs []string
	for _, hash := range stdlibHashes {
		if hash != toolHashes[0] {
			diffs = append(diffs, strings.Join(getGoOptions(t, toolHashes[0], hash), ", "))
		}
	}
	if len(diffs) != 0 {
		t.Errorf("a stdlib reachable from //:tool_user is not built in the configuration of //:plain, differing in: %s",
			strings.Join(diffs, "; "))
	}
}

// configHashes returns the hashes of the configurations of the targets matched
// by the given cquery expression.
func configHashes(t *testing.T, query string, flags ...string) []string {
	// See nogoGoOptions for why the build is necessary.
	if err := bazel_testing.RunBazel(append([]string{"build", "//:plain", "--nobuild"}, flags...)...); err != nil {
		t.Fatalf("bazel build //:plain: %v", err)
	}
	out, err := bazel_testing.BazelOutput(append(
		[]string{"cquery", "--output=jsonproto", query},
		flags...,
	)...)
	if err != nil {
		t.Fatalf("bazel cquery '%s': %v", query, err)
	}
	return extractConfigHashes(t, bytes.TrimSpace(out))
}
