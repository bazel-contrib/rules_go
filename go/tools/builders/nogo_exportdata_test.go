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
	"fmt"
	"strings"
	"testing"
)

// arEntry formats one ar archive entry, including its padding byte.
func arEntry(name, content string) string {
	hdr := fmt.Sprintf("%-16s%-12s%-6s%-6s%-8s%-10d`\n", name, "0", "0", "0", "644", len(content))
	if len(content)%2 == 1 {
		content += "\n"
	}
	return hdr + content
}

const testPkgDef = "go object linux amd64 go1.26 X:none\n" +
	"build id \"abc/def\"\n" +
	"\n" +
	"$$B\n" +
	"u\x00\x01payload\n$$" + // export data may contain "\n" and "$$"
	"\n$$\n"

const testExportData = "u\x00\x01payload\n$$"

func TestExtractExportData(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    string
		wantErr string
	}{
		{
			name:  "pkgdef only",
			input: exportDataArMagic + arEntry("__.PKGDEF", testPkgDef),
			want:  testExportData,
		},
		{
			name:  "pkgdef after other entries",
			input: exportDataArMagic + arEntry("_go_.o", "odd") + arEntry("__.PKGDEF", testPkgDef),
			want:  testExportData,
		},
		{
			name:  "gnu style name",
			input: exportDataArMagic + arEntry("__.PKGDEF/", testPkgDef),
			want:  testExportData,
		},
		{
			name:  "not an archive",
			input: "ibare export data",
			want:  "ibare export data",
		},
		{
			name:    "no pkgdef",
			input:   exportDataArMagic + arEntry("_go_.o", "data"),
			wantErr: "no __.PKGDEF entry",
		},
		{
			name:    "truncated header",
			input:   exportDataArMagic + "__.PKGDEF",
			wantErr: "truncated archive header",
		},
		{
			name:    "size past end",
			input:   exportDataArMagic + arEntry("__.PKGDEF", testPkgDef)[:exportDataArHeaderLen+5],
			wantErr: "invalid archive entry size",
		},
		{
			name:    "missing object header",
			input:   exportDataArMagic + arEntry("__.PKGDEF", "$$B\nu\n$$\n"),
			wantErr: "object header",
		},
		{
			name:    "old section start",
			input:   exportDataArMagic + arEntry("__.PKGDEF", "go object x\n$$\ni\n$$\n"),
			wantErr: "unknown export data header",
		},
		{
			name:    "missing end marker",
			input:   exportDataArMagic + arEntry("__.PKGDEF", "go object x\n$$B\nudata"),
			wantErr: "end-of-section marker",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := extractExportData([]byte(tt.input))
			if tt.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("got error %v, want error containing %q", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if string(got) != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}
