//go:build go1.18 && !go1.21
// +build go1.18,!go1.21

package main

import "testing"

func TestNormalizeGoVersionPre121(t *testing.T) {
	testCases := []struct {
		name      string
		goVersion string
		want      string
	}{
		{name: "patch release", goVersion: "1.20.14", want: "go1.20"},
		{name: "rc prerelease", goVersion: "1.20rc1", want: "go1.20"},
		{name: "custom suffix", goVersion: "1.26-abcdef", want: "go1.26"},
		{name: "custom patch suffix", goVersion: "go1.26.3-custom", want: "go1.26"},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			if got := normalizeGoVersion(tc.goVersion); got != tc.want {
				t.Fatalf("normalizeGoVersion(%q) = %q, want %q", tc.goVersion, got, tc.want)
			}
		})
	}
}
