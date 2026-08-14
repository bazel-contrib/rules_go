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
		Fragments []struct {
			Name    string            `json:"name"`
			Options map[string]string `json:"options"`
		} `json:"fragmentOptions"`
	}
	if err := json.Unmarshal(bytes.TrimSpace(out), &jsonOut); err != nil {
		t.Fatalf("Failed to decode bazel config JSON output %v: %q", err, string(out))
	}
	// The repository part of the keys depends on the canonical repository name,
	// so only keep the part that identifies the setting.
	const configPkg = "//go/config:"
	var goOptions []string
	for _, fragment := range jsonOut.Fragments {
		if fragment.Name != "user-defined" {
			continue
		}
		for key, value := range fragment.Options {
			if _, name, found := strings.Cut(key, configPkg); found {
				goOptions = append(goOptions, fmt.Sprintf("%s=%s", name, value))
			}
		}
	}
	sort.Strings(goOptions)
	return goOptions
}
