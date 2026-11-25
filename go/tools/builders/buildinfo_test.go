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
			name:   "normalizes version and strips non-checksum qualifiers",
			purl:   "pkg:golang/example.com/module@1.2.3?goos=linux#cmd/tool",
			want:   moduleInfo{path: "example.com/module", version: "v1.2.3"},
			wantOK: true,
		},
		{
			name:   "preserves checksum qualifier",
			purl:   "pkg:golang/example.com/module@1.2.3?checksum=h1:AbCd%2FEfGh%2BIjKl%3D",
			want:   moduleInfo{path: "example.com/module", version: "v1.2.3", sum: "h1:AbCd/EfGh+IjKl="},
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
		{path: "example.com/module", version: "1.2.3", sum: "h1:aaa="},
		{path: "example.com/module", version: "1.2.3", sum: "h1:aaa="},
		{path: "example.com/module", version: "1.2.3", sum: "h1:bbb="},
		{path: "", version: "v1.0.0"},
		{path: "example.com/missing/version", version: ""},
	}, moduleInfo{})

	got := make([]string, 0, len(deps))
	for _, dep := range deps {
		got = append(got, dep.Path+"@"+dep.Version+";"+dep.Sum)
	}
	want := []string{
		"example.com/module@1.2.3;h1:aaa=",
		"example.com/module@1.2.3;h1:bbb=",
		"github.com/google/go-cmp@v0.6.0;",
		"golang.org/x/text@v0.15.0;",
		"golang.org/x/text@v0.16.0;",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got deps %v; want %v", got, want)
	}
}

func TestModInfoDataRoundTrip(t *testing.T) {
	settings := []debug.BuildSetting{
		{Key: "-buildmode", Value: "exe"},
		{Key: "-compiler", Value: "gc"},
		{Key: "GOARCH", Value: "arm64"},
	}
	info := parseModInfoData(t, modInfoData("example.com/cmd/tool", moduleInfo{}, settings, []moduleInfo{
		{path: "golang.org/x/sync", version: "v0.8.0", sum: "h1:3NFvSEYkUoMifnESzZl15y791HH1qU2xm6eCJU5ZPXQ="},
		{path: "github.com/google/go-cmp", version: "v0.6.0", sum: "h1:ofyhxvXcZhMsU5ulbFiLKl/XBFqE1GSq7atu8tAmTRI="},
		{path: "github.com/google/go-cmp", version: "v0.6.0", sum: "h1:ofyhxvXcZhMsU5ulbFiLKl/XBFqE1GSq7atu8tAmTRI="},
	}))

	if info.Path != "example.com/cmd/tool" {
		t.Fatalf("got Path %q; want %q", info.Path, "example.com/cmd/tool")
	}
	if info.Main.Path != "" || info.Main.Version != "" {
		t.Fatalf("got Main %+v; want empty", info.Main)
	}

	got := make([]string, 0, len(info.Deps))
	for _, dep := range info.Deps {
		got = append(got, dep.Path+"@"+dep.Version+";"+dep.Sum)
	}
	want := []string{
		"github.com/google/go-cmp@v0.6.0;h1:ofyhxvXcZhMsU5ulbFiLKl/XBFqE1GSq7atu8tAmTRI=",
		"golang.org/x/sync@v0.8.0;h1:3NFvSEYkUoMifnESzZl15y791HH1qU2xm6eCJU5ZPXQ=",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got deps %v; want %v", got, want)
	}
	if !reflect.DeepEqual(info.Settings, settings) {
		t.Fatalf("got settings %+v; want %+v", info.Settings, settings)
	}
}

func TestModInfoDataMainModule(t *testing.T) {
	info := parseModInfoData(t, modInfoData(
		"example.com/cmd/tool",
		moduleInfo{path: "example.com/main", version: "v1.2.3"},
		nil,
		[]moduleInfo{
			{path: "example.com/main", version: "v1.2.3"},
			{path: "example.com/main", version: "v1.1.0"},
			{path: "example.com/dep", version: "v0.1.0"},
		},
	))

	if info.Main.Path != "example.com/main" || info.Main.Version != "v1.2.3" {
		t.Fatalf("got Main %+v; want example.com/main@v1.2.3", info.Main)
	}
	if len(info.Deps) != 1 || info.Deps[0].Path != "example.com/dep" {
		t.Fatalf("got Deps %+v; want only example.com/dep", info.Deps)
	}
}

func TestModInfoDataWithoutDeps(t *testing.T) {
	info := parseModInfoData(t, modInfoData("example.com/cmd/tool", moduleInfo{}, nil, nil))
	if info.Path != "example.com/cmd/tool" {
		t.Fatalf("got Path %q; want %q", info.Path, "example.com/cmd/tool")
	}
	if len(info.Deps) != 0 {
		t.Fatalf("got %d deps; want 0", len(info.Deps))
	}
}

func TestModInfoDataFormat(t *testing.T) {
	got := modInfoData("example.com/cmd/tool", moduleInfo{}, []debug.BuildSetting{
		{Key: "-buildmode", Value: "exe"},
		{Key: "-tags", Value: "foo,bar"},
	}, []moduleInfo{
		{path: "github.com/google/go-cmp", version: "v0.6.0"},
		{path: "golang.org/x/sync", version: "v0.8.0"},
	})
	want := buildInfoStart +
		"path\texample.com/cmd/tool\n" +
		"dep\tgithub.com/google/go-cmp\tv0.6.0\t\n" +
		"dep\tgolang.org/x/sync\tv0.8.0\t\n" +
		"build\t-buildmode=exe\n" +
		"build\t-tags=foo,bar\n" +
		buildInfoEnd
	if got != want {
		t.Fatalf("got %q; want %q", got, want)
	}
}

func TestModInfoDataWithoutPathOrDeps(t *testing.T) {
	info := parseModInfoData(t, modInfoData("", moduleInfo{}, nil, nil))
	if info.Path != "" {
		t.Fatalf("got Path %q; want empty", info.Path)
	}
	if len(info.Deps) != 0 {
		t.Fatalf("got %d deps; want 0", len(info.Deps))
	}
	if len(info.Settings) != 0 {
		t.Fatalf("got %d settings; want 0", len(info.Settings))
	}
}

func TestBuildInfoSettingsFromEnv(t *testing.T) {
	env := map[string]string{
		"CGO_ENABLED":  "1",
		"GOARCH":       "amd64",
		"GOEXPERIMENT": "rangefunc",
		"GOOS":         "linux",
		"GOAMD64":      "v3",
	}
	got := buildInfoSettingsFromEnv("pie", []string{"foo", "race", "bar"}, true, false, true, false, func(key string) string {
		return env[key]
	})
	want := []debug.BuildSetting{
		{Key: "-buildmode", Value: "pie"},
		{Key: "-compiler", Value: "gc"},
		{Key: "-cover", Value: "true"},
		{Key: "-race", Value: "true"},
		{Key: "-tags", Value: "foo,bar"},
		{Key: "-trimpath", Value: "true"},
		{Key: "CGO_ENABLED", Value: "1"},
		{Key: "GOARCH", Value: "amd64"},
		{Key: "GOEXPERIMENT", Value: "rangefunc"},
		{Key: "GOOS", Value: "linux"},
		{Key: "GOAMD64", Value: "v3"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got settings %+v; want %+v", got, want)
	}
}

func TestBuildInfoSettingsPreservesExplicitInstrumentationTags(t *testing.T) {
	got := buildInfoSettingsFromEnv("normal", []string{"race", "foo", "race"}, true, false, false, false, func(string) string {
		return ""
	})
	want := []debug.BuildSetting{
		{Key: "-buildmode", Value: "exe"},
		{Key: "-compiler", Value: "gc"},
		{Key: "-race", Value: "true"},
		{Key: "-tags", Value: "race,foo"},
		{Key: "-trimpath", Value: "true"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got settings %+v; want %+v", got, want)
	}
}

func TestBuildInfoArchSetting(t *testing.T) {
	testCases := []struct {
		name              string
		goarch            string
		go123ArchSettings bool
		env               map[string]string
		wantKey           string
		wantValue         string
	}{
		{name: "amd64 default", goarch: "amd64", wantKey: "GOAMD64", wantValue: "v1"},
		{name: "arm default unknown", goarch: "arm"},
		{name: "arm android default", goarch: "arm", env: map[string]string{"GOOS": "android"}, wantKey: "GOARM", wantValue: "7"},
		{name: "arm explicit", goarch: "arm", env: map[string]string{"GOARM": "6"}, wantKey: "GOARM", wantValue: "6"},
		{name: "arm64 before Go 1.23", goarch: "arm64"},
		{name: "arm64 from Go 1.23", goarch: "arm64", go123ArchSettings: true, wantKey: "GOARM64", wantValue: "v8.0"},
		{name: "riscv64 before Go 1.23", goarch: "riscv64"},
		{name: "riscv64 from Go 1.23", goarch: "riscv64", go123ArchSettings: true, wantKey: "GORISCV64", wantValue: "rva20u64"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			gotKey, gotValue := buildInfoArchSetting(tc.goarch, tc.go123ArchSettings, func(key string) string {
				return tc.env[key]
			})
			if gotKey != tc.wantKey || gotValue != tc.wantValue {
				t.Fatalf("got %q=%q; want %q=%q", gotKey, gotValue, tc.wantKey, tc.wantValue)
			}
		})
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
