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

import "testing"

func TestEncodeResponseFileArg(t *testing.T) {
	for _, tc := range []struct {
		name string
		arg  string
		want string
	}{
		{
			name: "empty",
			arg:  "",
			want: `""`,
		},
		{
			name: "unchanged without special characters",
			arg:  "-pthread",
			want: "-pthread",
		},
		{
			name: "keeps whitespace in one argument",
			arg:  "-target x86_64-linux-gnu --sysroot=/dev/null",
			want: `"-target x86_64-linux-gnu --sysroot=/dev/null"`,
		},
		{
			name: "escapes special characters",
			arg:  `-Wl,-rpath,$ORIGIN -X "quoted" C:\tmp\lib`,
			want: `"-Wl,-rpath,\$ORIGIN -X \"quoted\" C:\\tmp\\lib"`,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := encodeResponseFileArg(tc.arg); got != tc.want {
				t.Fatalf("encodeResponseFileArg(%q) = %q; want %q", tc.arg, got, tc.want)
			}
		})
	}
}
