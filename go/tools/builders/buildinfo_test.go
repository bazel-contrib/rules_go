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

package main

import (
	"runtime/debug"
	"slices"
	"strings"
	"testing"
)

func parseModInfoData(t *testing.T, data string) *debug.BuildInfo {
	t.Helper()

	info, found := strings.CutPrefix(data, buildInfoStart)
	if !found {
		t.Fatalf("modinfo missing start marker: %q", data)
	}
	info, found = strings.CutSuffix(info, buildInfoEnd)
	if !found {
		t.Fatalf("modinfo missing end marker: %q", data)
	}

	parsed, err := debug.ParseBuildInfo(info)
	if err != nil {
		t.Fatalf("ParseBuildInfo(%q): %v", info, err)
	}
	return parsed
}

func TestBuildInfoDepsSortAndDedup(t *testing.T) {
	deps := buildInfoDeps([]moduleInfo{
		{path: "golang.org/x/text", version: "v0.15.0"},
		{path: "github.com/google/go-cmp", version: "v0.6.0"},
		{path: "golang.org/x/text", version: "v0.15.0"},
		{path: "", version: "v1.0.0"},
		{path: "example.com/missing/version", version: ""},
	})

	got := make([]string, 0, len(deps))
	for _, dep := range deps {
		got = append(got, dep.Path+"@"+dep.Version)
	}
	want := []string{
		"github.com/google/go-cmp@v0.6.0",
		"golang.org/x/text@v0.15.0",
	}
	if !slices.Equal(got, want) {
		t.Fatalf("got deps %v; want %v", got, want)
	}
}

func TestModInfoDataRoundTrip(t *testing.T) {
	info := parseModInfoData(t, modInfoData([]moduleInfo{
		{path: "golang.org/x/sync", version: "v0.8.0"},
		{path: "github.com/google/go-cmp", version: "v0.6.0"},
		{path: "github.com/google/go-cmp", version: "v0.6.0"},
	}))

	if info.Path != "" {
		t.Fatalf("got Path %q; want empty", info.Path)
	}
	if info.Main.Path != "" || info.Main.Version != "" {
		t.Fatalf("got Main %+v; want empty", info.Main)
	}

	got := make([]string, 0, len(info.Deps))
	for _, dep := range info.Deps {
		got = append(got, dep.Path+"@"+dep.Version)
	}
	want := []string{
		"github.com/google/go-cmp@v0.6.0",
		"golang.org/x/sync@v0.8.0",
	}
	if !slices.Equal(got, want) {
		t.Fatalf("got deps %v; want %v", got, want)
	}
}

func TestModInfoDataWithoutDeps(t *testing.T) {
	info := parseModInfoData(t, modInfoData(nil))
	if len(info.Deps) != 0 {
		t.Fatalf("got %d deps; want 0", len(info.Deps))
	}
}

func TestModInfoDataFormat(t *testing.T) {
	got := modInfoData([]moduleInfo{
		{path: "github.com/google/go-cmp", version: "v0.6.0"},
		{path: "golang.org/x/sync", version: "v0.8.0"},
	})
	want := buildInfoStart +
		"dep\tgithub.com/google/go-cmp\tv0.6.0\t\n" +
		"dep\tgolang.org/x/sync\tv0.8.0\t\n" +
		buildInfoEnd
	if got != want {
		t.Fatalf("got %q; want %q", got, want)
	}
}

func TestShouldEmitBuildInfo(t *testing.T) {
	testCases := []struct {
		name      string
		goVersion string
		buildmode string
		want      bool
	}{
		{name: "default version", goVersion: "", want: true},
		{name: "go117", goVersion: "1.17", want: false},
		{name: "go117_patch", goVersion: "1.17.1", want: false},
		{name: "go117_rc", goVersion: "1.17rc1", want: false},
		{name: "go118", goVersion: "1.18", want: true},
		{name: "go_prefix", goVersion: "go1.18.3", want: true},
		{name: "plugin", goVersion: "1.24.0", buildmode: "plugin", want: false},
		{name: "unknown", goVersion: "devel go1.26-abcdef", want: true},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			if got := shouldEmitBuildInfo(tc.goVersion, tc.buildmode); got != tc.want {
				t.Fatalf("shouldEmitBuildInfo(%q, %q) = %t; want %t", tc.goVersion, tc.buildmode, got, tc.want)
			}
		})
	}
}
