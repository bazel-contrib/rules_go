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

package runfiles

import "testing"

func TestLookupFunctionRepository(t *testing.T) {
	packageRepos := map[string]string{
		"main":                   "my_repo",
		"example.com/foo/bar":    "foo_repo",
		"example.com/foo.v2":     "foo_v2_repo",
		"example.com/foo.v2/pkg": "foo_v2_repo",
		"pkg":                    "",
	}
	for _, tc := range []struct {
		function string
		want     string
		wantOk   bool
	}{
		{"main.main", "my_repo", true},
		{"main.main.func1", "my_repo", true},
		{"example.com/foo/bar.F", "foo_repo", true},
		{"example.com/foo/bar.F.func2", "foo_repo", true},
		{"example.com/foo/bar.(*T).M", "foo_repo", true},
		{"example.com/foo/bar.T.M", "foo_repo", true},
		{"example.com/foo.v2.F", "foo_v2_repo", true},
		{"example.com/foo.v2.(*T).M", "foo_v2_repo", true},
		{"example.com/foo.v2/pkg.F", "foo_v2_repo", true},
		{"example.com/foo/bar.F[go.shape.int]", "foo_repo", true},
		{"example.com/foo/bar.F[example.com/other/pkg.T]", "foo_repo", true},
		{"example.com/foo/bar.(*T[go.shape.string]).M", "foo_repo", true},
		{"pkg.F", "", true},
		{"runtime.gopanic", "", false},
		{"example.com/unknown.F", "", false},
		{"", "", false},
		{"noPackage", "", false},
	} {
		got, ok := lookupFunctionRepository(tc.function, packageRepos)
		if got != tc.want || ok != tc.wantOk {
			t.Errorf("lookupFunctionRepository(%q) = %q, %t; want %q, %t", tc.function, got, ok, tc.want, tc.wantOk)
		}
	}
}
