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
	"slices"
	"testing"
)

func TestReadFIPSPackageList(t *testing.T) {
	for _, tc := range []struct {
		desc, content string
		want          []string
	}{
		{
			desc:    "packages",
			content: "crypto/internal/fips140/v1.0.0-c2097c7c\ncrypto/internal/fips140/v1.0.0-c2097c7c/aes\n",
			want: []string{
				"crypto/internal/fips140/v1.0.0-c2097c7c",
				"crypto/internal/fips140/v1.0.0-c2097c7c/aes",
			},
		},
		{
			desc:    "blank lines",
			content: "\n\ncrypto/internal/fips140/v1.0.0/aes\n\n",
			want:    []string{"crypto/internal/fips140/v1.0.0/aes"},
		},
		{
			desc: "empty",
		},
	} {
		t.Run(tc.desc, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "fips_packages.txt")
			if err := os.WriteFile(path, []byte(tc.content), 0o644); err != nil {
				t.Fatal(err)
			}
			got, err := readFIPSPackageList(path)
			if err != nil {
				t.Fatal(err)
			}
			if !slices.Equal(got, tc.want) {
				t.Errorf("got %v, want %v", got, tc.want)
			}
		})
	}
}

func TestReadFIPSPackageListMissingFile(t *testing.T) {
	if _, err := readFIPSPackageList(filepath.Join(t.TempDir(), "missing.txt")); err == nil {
		t.Error("got nil error, want an error")
	}
}
