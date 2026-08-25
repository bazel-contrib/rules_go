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
	"reflect"
	"testing"
)

func TestParseCoverProfile(t *testing.T) {
	for _, test := range []struct {
		desc    string
		profile string
		want    map[string][]int
	}{
		{
			desc: "a block covers every line it spans",
			profile: "mode: set\n" +
				"pkg/sample.go:3.24,4.11 1 0\n" +
				"pkg/sample.go:4.11,6.3 1 0\n",
			want: map[string][]int{"pkg/sample.go": {3, 4, 5, 6}},
		},
		{
			desc: "overlapping blocks report each line once",
			profile: "mode: set\n" +
				"a.go:1.1,3.2 1 0\n" +
				"a.go:2.1,4.2 1 0\n",
			want: map[string][]int{"a.go": {1, 2, 3, 4}},
		},
		{
			desc: "blocks are grouped by source file",
			profile: "mode: set\n" +
				"a.go:1.1,1.2 1 0\n" +
				"b.go:7.1,7.2 1 0\n",
			want: map[string][]int{"a.go": {1}, "b.go": {7}},
		},
		{
			// covdata writes the path unescaped, so splitting the line on
			// whitespace would take "sp" for the whole record.
			desc:    "a path containing a space is read whole",
			profile: "mode: set\nsp ace/sample.go:5.26,6.11 1 0\n",
			want:    map[string][]int{"sp ace/sample.go": {5, 6}},
		},
		{
			desc:    "a path containing a colon is split at the last one",
			profile: "mode: set\nweird:dir/sample.go:2.1,2.9 1 0\n",
			want:    map[string][]int{"weird:dir/sample.go": {2}},
		},
		{
			// The header check runs only once the record pattern has failed,
			// so a path that looks like the header is not mistaken for it.
			desc:    "a path beginning with mode: is a record",
			profile: "mode: set\nmode:x.go:2.1,2.9 1 0\n",
			want:    map[string][]int{"mode:x.go": {2}},
		},
		{
			// covdata leaves a trailing carriage return on Windows.
			desc:    "a carriage return does not defeat the line anchor",
			profile: "mode: set\r\na.go:1.1,2.2 1 0\r\n",
			want:    map[string][]int{"a.go": {1, 2}},
		},
		{
			desc:    "an empty profile has no records",
			profile: "",
			want:    map[string][]int{},
		},
		{
			desc:    "a header alone has no records",
			profile: "mode: set\n",
			want:    map[string][]int{},
		},
		{
			desc:    "blank lines are skipped",
			profile: "mode: set\n\na.go:1.1,1.2 1 0\n\n",
			want:    map[string][]int{"a.go": {1}},
		},
	} {
		t.Run(test.desc, func(t *testing.T) {
			got, err := parseCoverProfile(test.profile)
			if err != nil {
				t.Fatalf("parseCoverProfile returned an unexpected error: %v", err)
			}
			if !reflect.DeepEqual(got, test.want) {
				t.Errorf("parseCoverProfile = %v, want %v", got, test.want)
			}
		})
	}
}

func TestParseCoverProfileRejectsMalformedInput(t *testing.T) {
	for _, test := range []struct {
		desc    string
		profile string
	}{
		{"a line with no block position", "mode: set\nnot a profile line\n"},
		{"a record missing its counts", "mode: set\na.go:1.1,2.2\n"},
		{"a record missing a column", "mode: set\na.go:1,2.2 1 0\n"},
		{"a non-numeric line number", "mode: set\na.go:x.1,2.2 1 0\n"},
		{"an empty path", "mode: set\n:1.1,2.2 1 0\n"},
		{"a block ending before it starts", "mode: set\na.go:5.1,2.2 1 0\n"},
		{"a block starting at line zero", "mode: set\na.go:0.1,2.2 1 0\n"},
	} {
		t.Run(test.desc, func(t *testing.T) {
			if _, err := parseCoverProfile(test.profile); err == nil {
				t.Errorf("parseCoverProfile accepted %q, want an error", test.profile)
			}
		})
	}
}
