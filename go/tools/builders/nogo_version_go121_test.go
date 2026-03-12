//go:build go1.21
// +build go1.21

package main

import "testing"

func TestNormalizeGoVersionGo121(t *testing.T) {
	testCases := []struct {
		name      string
		goVersion string
		want      string
	}{
		{name: "patch release", goVersion: "1.20.14", want: "go1.20.14"},
		{name: "release unchanged", goVersion: "go1.25.3", want: "go1.25.3"},
		{name: "beta unchanged", goVersion: "1.26beta1", want: "go1.26beta1"},
		{name: "rc unchanged", goVersion: "1.26rc2", want: "go1.26rc2"},
		{name: "custom suffix", goVersion: "1.26-abcdef", want: "go1.26"},
		{name: "custom patch suffix", goVersion: "go1.26.3-custom", want: "go1.26.3"},
		{name: "custom rc suffix", goVersion: "go1.26rc1-custom", want: "go1.26rc1"},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			if got := normalizeGoVersion(tc.goVersion); got != tc.want {
				t.Fatalf("normalizeGoVersion(%q) = %q, want %q", tc.goVersion, got, tc.want)
			}
		})
	}
}
