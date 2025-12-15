// Copyright 2023 The Bazel Authors. All rights reserved.
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
	"reflect"
	"testing"
)

func TestParseNolint(t *testing.T) {
	tests := []struct {
		Name    string
		Comment string
		Valid   bool
		Linters []string
	}{
		{
			Name:    "Invalid",
			Comment: "not a comment",
		},
		{
			Name:    "No match",
			Comment: "// comment",
		},
		{
			Name:    "All linters",
			Comment: "//nolint",
			Valid:   true,
		},
		{
			Name:    "All linters (explicit)",
			Comment: "//nolint:all",
			Valid:   true,
		},
		{
			Name:    "Single linter",
			Comment: "// nolint:foo",
			Valid:   true,
			Linters: []string{"foo"},
		},
		{
			Name:    "Single linter with an explanation",
			Comment: "//nolint:foo // the foo lint is invalid for this line",
			Valid:   true,
			Linters: []string{"foo"},
		},
		{
			Name:    "Multiple linters",
			Comment: "// nolint:a,b,c",
			Valid:   true,
			Linters: []string{"a", "b", "c"},
		},
		{
			Name:    "Multiple linters with explanation",
			Comment: "// nolint:a,b,c // some reason",
			Valid:   true,
			Linters: []string{"a", "b", "c"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.Name, func(t *testing.T) {
			result, ok := parseNolint(tc.Comment)
			if tc.Valid != ok {
				t.Fatalf("parseNolint expect %t got %t", tc.Valid, ok)
			}
			var linters map[string]bool
			if len(tc.Linters) != 0 {
				linters = make(map[string]bool)
				for _, l := range tc.Linters {
					linters[l] = true
				}
			}
			if !reflect.DeepEqual(result, linters) {
				t.Fatalf("parseNolint expect %v got %v", linters, result)
			}
		})
	}
}
