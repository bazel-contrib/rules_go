/* Copyright 2016 The Bazel Authors. All rights reserved.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

   http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package main

import (
	"go/build"
	"io/ioutil"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"testing"
)

var testfiles = map[string]string{
	"cgo.go": `
//+build cgo

package tags

/*
#include <stdio.h>
#include <stdlib.h>

void myprint(char* s) {
		printf("%s", s);
}
*/

import "C"

func main() {
	C.myprint("hello")
}
`,
	"extra.go": `
//+build a,b b,c

package tags
`,
	"ignore.go": `
//+build ignore

package tags
`,
	"normal.go": `
package tags
`,
	"on_darwin.go": `
package tags
`,
	"system.go": `
//+build arm,darwin linux,amd64

package tags
`,
}

func TestTags(t *testing.T) {
	tempdir, err := ioutil.TempDir("", "goruletest")
	if err != nil {
		t.Fatalf("Error creating temporary directory: %v", err)
	}
	defer os.RemoveAll(tempdir)

	input := []string{}
	for k, v := range testfiles {
		p := filepath.Join(tempdir, k)
		if err := ioutil.WriteFile(p, []byte(v), 0644); err != nil {
			t.Fatalf("WriteFile(%s): %v", p, err)
		}
		input = append(input, k)
	}
	sort.Strings(input)

	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd: %v", err)
	}

	err = os.Chdir(tempdir)
	if err != nil {
		t.Fatalf("Chdir(%s): %v", tempdir, err)
	}
	defer os.Chdir(wd)

	bctx := build.Default
	// Always fake the os and arch
	bctx.GOOS = "darwin"
	bctx.GOARCH = "amd64"
	bctx.CgoEnabled = false
	runTest(t, bctx, input, []string{"normal.go", "on_darwin.go"})
	bctx.GOOS = "linux"
	runTest(t, bctx, input, []string{"normal.go", "system.go"})
	bctx.GOARCH = "arm"
	runTest(t, bctx, input, []string{"normal.go"})
	bctx.BuildTags = []string{"a", "b"}
	runTest(t, bctx, input, []string{"extra.go", "normal.go"})
	bctx.BuildTags = []string{"a", "c"}
	runTest(t, bctx, input, []string{"normal.go"})
	bctx.CgoEnabled = true
	runTest(t, bctx, input, []string{"cgo.go", "normal.go"})
}

func runTest(t *testing.T, bctx build.Context, inputs []string, expect []string) {
	build.Default = bctx
	got, err := filterAndSplitFiles(inputs)
	if err != nil {
		t.Errorf("filter %v,%v,%v,%v failed: %v", bctx.GOOS, bctx.GOARCH, bctx.CgoEnabled, bctx.BuildTags, err)
	}
	gotGoFilenames := make([]string, len(got.goSrcs))
	for i, src := range got.goSrcs {
		gotGoFilenames[i] = src.filename
	}
	if !reflect.DeepEqual(expect, gotGoFilenames) {
		t.Errorf("filter %v,%v,%v,%v: expect %v got %v", bctx.GOOS, bctx.GOARCH, bctx.CgoEnabled, bctx.BuildTags, expect, got)
	}
}

func TestApplyTestFilter(t *testing.T) {
	inputs := []fileInfo{
		{filename: "lib.go", pkg: "example"},
		{filename: "internal_test.go", pkg: "example"},
		{filename: "external_test.go", pkg: "example_test"},
	}
	for _, tc := range []struct {
		name       string
		testFilter string
		want       []string
	}{
		{
			name:       "off",
			testFilter: "off",
			want:       []string{"lib.go", "internal_test.go", "external_test.go"},
		},
		{
			name:       "only",
			testFilter: "only",
			want:       []string{"external_test.go"},
		},
		{
			name:       "exclude",
			testFilter: "exclude",
			want:       []string{"lib.go", "internal_test.go"},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			srcs := archiveSrcs{goSrcs: append([]fileInfo(nil), inputs...)}
			if err := applyTestFilter(tc.testFilter, &srcs); err != nil {
				t.Fatalf("applyTestFilter(%q): %v", tc.testFilter, err)
			}
			got := make([]string, len(srcs.goSrcs))
			for i, src := range srcs.goSrcs {
				got[i] = src.filename
			}
			if !reflect.DeepEqual(got, tc.want) {
				t.Errorf("applyTestFilter(%q): got %v, want %v", tc.testFilter, got, tc.want)
			}
		})
	}
}

// abs is a dummy env.go abs to avoid depending on env.go and flags.go.
func abs(p string) string {
	return p
}
