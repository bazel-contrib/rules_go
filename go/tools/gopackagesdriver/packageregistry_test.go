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
	"strings"
	"testing"
)

func TestMatchImportPaths(t *testing.T) {
	pr := NewPackageRegistry(bazelVersion{},
		&FlatPackage{ID: "//lib:go_default_library", Name: "lib", PkgPath: "example.com/lib"},
		&FlatPackage{ID: "//lib/sub:go_default_library", Name: "sub", PkgPath: "example.com/lib/sub"},
		&FlatPackage{ID: "//other:go_default_library", Name: "other", PkgPath: "example.com/other"},
		&FlatPackage{ID: "//stdlib:fmt", Name: "fmt", PkgPath: "fmt", Standard: true},
	)

	t.Run("exact", func(t *testing.T) {
		ids, err := pr.MatchImportPaths([]string{"example.com/lib"})
		if err != nil {
			t.Fatal(err)
		}
		if len(ids) != 1 || ids[0] != "//lib:go_default_library" {
			t.Fatalf("got %v", ids)
		}
	})

	t.Run("prefix", func(t *testing.T) {
		ids, err := pr.MatchImportPaths([]string{"example.com/lib/..."})
		if err != nil {
			t.Fatal(err)
		}
		want := "//lib/sub:go_default_library //lib:go_default_library"
		if got := strings.Join(ids, " "); got != want {
			t.Fatalf("got %q, want %q", got, want)
		}
	})

	t.Run("unknown", func(t *testing.T) {
		_, err := pr.MatchImportPaths([]string{"example.com/missing"})
		if err == nil || !strings.Contains(err.Error(), "found no packages matching import path") {
			t.Fatalf("got %v", err)
		}
	})

	t.Run("stdlib_not_root", func(t *testing.T) {
		_, err := pr.MatchImportPaths([]string{"fmt"})
		if err == nil {
			t.Fatal("expected error matching stdlib as root")
		}
	})

	t.Run("file_query", func(t *testing.T) {
		_, err := pr.MatchImportPaths([]string{"file=lib.go"})
		if err == nil || !strings.Contains(err.Error(), "not supported") {
			t.Fatalf("got %v", err)
		}
	})

	t.Run("local_query", func(t *testing.T) {
		_, err := pr.MatchImportPaths([]string{"./lib"})
		if err == nil || !strings.Contains(err.Error(), "not supported") {
			t.Fatalf("got %v", err)
		}
	})
}
