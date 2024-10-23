//go:build !windows

package main

import "testing"

func TestVerbFromName(t *testing.T) {
	testCases := []struct {
		name string
		verb string
	}{
		{"/a/b/c/d/builder", ""},
		{"builder", ""},
		{"/a/b/c/d/builder-cc", "cc"},
		{"builder-ld", "ld"},
		{"c:\\builder\\builder.exe", ""},
		{"c:\\builder with spaces\\builder-cc.exe", "cc"},
	}

	for _, tc := range testCases {
		result := verbFromName(tc.name)
		if result != tc.verb {
			t.Fatalf("retrieved invalid verb %q from name %q", result, tc.name)
		}
	}
}
