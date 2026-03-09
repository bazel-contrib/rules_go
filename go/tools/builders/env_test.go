//go:build !windows

package main

import (
	"reflect"
	"strings"
	"testing"
)

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

func TestTransformArgs(t *testing.T) {
	upper := func(s string) string { return strings.ToUpper(s) }

	testCases := []struct {
		name     string
		args     []string
		flags    []string
		expected []string
	}{
		{
			name:     "space-separated",
			args:     []string{"-isystem", "relative/path"},
			flags:    []string{"-isystem"},
			expected: []string{"-isystem", "RELATIVE/PATH"},
		},
		{
			name:     "equals-separated",
			args:     []string{"-isystem=relative/path"},
			flags:    []string{"-isystem"},
			expected: []string{"-isystem=RELATIVE/PATH"},
		},
		{
			name:     "concatenated",
			args:     []string{"-Irelative/path"},
			flags:    []string{"-I"},
			expected: []string{"-IRELATIVE/PATH"},
		},
		{
			name:     "xclang forwarding",
			args:     []string{"-Xclang", "-internal-isystem", "-Xclang", "relative/path"},
			flags:    []string{"-internal-isystem"},
			expected: []string{"-Xclang", "-internal-isystem", "-Xclang", "RELATIVE/PATH"},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			args := make([]string, len(tc.args))
			copy(args, tc.args)
			transformArgs(args, tc.flags, upper)
			if !reflect.DeepEqual(args, tc.expected) {
				t.Errorf("got %v, want %v", args, tc.expected)
			}
		})
	}
}
