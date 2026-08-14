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
	"os"
	"path/filepath"
	"reflect"
	"runtime/debug"
	"strings"
	"testing"
)

func TestModuleFromPURL(t *testing.T) {
	testCases := []struct {
		name    string
		purl    string
		want    moduleInfo
		wantOK  bool
		wantErr bool
	}{
		{
			name:   "versioned",
			purl:   "pkg:golang/github.com/google/go-cmp@v0.6.0",
			want:   moduleInfo{path: "github.com/google/go-cmp", version: "v0.6.0"},
			wantOK: true,
		},
		{
			name:   "versionless",
			purl:   "pkg:golang/example.com/versionless",
			want:   moduleInfo{path: "example.com/versionless", version: "(devel)"},
			wantOK: true,
		},
		{
			name:   "normalizes version and strips qualifiers",
			purl:   "pkg:golang/example.com/module@1.2.3?goos=linux#cmd/tool",
			want:   moduleInfo{path: "example.com/module", version: "v1.2.3"},
			wantOK: true,
		},
		{
			name:   "preserves commit hash",
			purl:   "pkg:golang/github.com/gorilla/context@234fd47e07d1004f0aed9c#api",
			want:   moduleInfo{path: "github.com/gorilla/context", version: "234fd47e07d1004f0aed9c"},
			wantOK: true,
		},
		{
			name:   "normalizes valid prerelease",
			purl:   "pkg:golang/example.com/module@1.2.3-alpha.1+build.01",
			want:   moduleInfo{path: "example.com/module", version: "v1.2.3-alpha.1+build.01"},
			wantOK: true,
		},
		{
			name:   "preserves invalid prerelease",
			purl:   "pkg:golang/example.com/module@1.2.3-alpha.01",
			want:   moduleInfo{path: "example.com/module", version: "1.2.3-alpha.01"},
			wantOK: true,
		},
		{
			name:   "preserves prerelease on shorthand",
			purl:   "pkg:golang/example.com/module@1.2-alpha",
			want:   moduleInfo{path: "example.com/module", version: "1.2-alpha"},
			wantOK: true,
		},
		{
			name:   "preserves build metadata on shorthand",
			purl:   "pkg:golang/example.com/module@1+meta",
			want:   moduleInfo{path: "example.com/module", version: "1+meta"},
			wantOK: true,
		},
		{
			name:   "unescapes components",
			purl:   "pkg:golang/example.com%2Fmodule@v1.2.3%2Bmeta",
			want:   moduleInfo{path: "example.com/module", version: "v1.2.3+meta"},
			wantOK: true,
		},
		{name: "non-Go package", purl: "pkg:npm/example@1.0.0"},
		{name: "empty Go module path", purl: "pkg:golang/", wantErr: true},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			got, ok, err := moduleFromPURL(tc.purl)
			if (err != nil) != tc.wantErr {
				t.Fatalf("moduleFromPURL(%q) error = %v; wantErr %t", tc.purl, err, tc.wantErr)
			}
			if ok != tc.wantOK {
				t.Fatalf("moduleFromPURL(%q) ok = %t; want %t", tc.purl, ok, tc.wantOK)
			}
			if got != tc.want {
				t.Fatalf("moduleFromPURL(%q) = %+v; want %+v", tc.purl, got, tc.want)
			}
		})
	}
}

func TestModulesFromPackageMetadataFiles(t *testing.T) {
	dir := t.TempDir()
	paths := []string{
		filepath.Join(dir, "cmp.json"),
		filepath.Join(dir, "versionless.json"),
		filepath.Join(dir, "non-go.json"),
	}
	contents := []string{
		`{"purl":"pkg:golang/github.com/google/go-cmp@v0.6.0"}`,
		`{"purl":"pkg:golang/example.com/versionless"}`,
		`{"purl":"pkg:npm/example@1.0.0"}`,
	}
	for i, path := range paths {
		if err := os.WriteFile(path, []byte(contents[i]), 0o600); err != nil {
			t.Fatal(err)
		}
	}

	got, err := modulesFromPackageMetadataFiles(paths)
	if err != nil {
		t.Fatal(err)
	}
	want := []moduleInfo{
		{path: "github.com/google/go-cmp", version: "v0.6.0"},
		{path: "example.com/versionless", version: "(devel)"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got modules %v; want %v", got, want)
	}
}

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
		{path: "golang.org/x/text", version: "v0.16.0"},
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
		"golang.org/x/text@v0.16.0",
	}
	if !reflect.DeepEqual(got, want) {
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
	if !reflect.DeepEqual(got, want) {
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
		buildmode string
		name      string
		want      bool
	}{
		{name: "default", want: true},
		{name: "c_archive", buildmode: "c-archive", want: false},
		{name: "c_shared", buildmode: "c-shared", want: false},
		{name: "plugin", buildmode: "plugin", want: false},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			if got := shouldEmitBuildInfo(tc.buildmode); got != tc.want {
				t.Fatalf("shouldEmitBuildInfo(%q) = %t; want %t", tc.buildmode, got, tc.want)
			}
		})
	}
}
