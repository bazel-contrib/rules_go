//go:build !windows

package main

import (
	"os"
	"path/filepath"
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
		{
			name:     "extern-C forwarding",
			args:     []string{"-Xclang", "-internal-externc-isystem", "-Xclang", "relative/path"},
			flags:    cgoAbsEnvFlags,
			expected: []string{"-Xclang", "-internal-externc-isystem", "-Xclang", "RELATIVE/PATH"},
		},
		{
			name:     "early C forwarding",
			args:     []string{"-Xpreprocessor", "-internal-externc-isystem", "-Xpreprocessor", "relative/path"},
			flags:    cgoAbsEnvFlags,
			expected: []string{"-Xpreprocessor", "-internal-externc-isystem", "-Xpreprocessor", "RELATIVE/PATH"},
		},
		{
			name:     "early C++ forwarding",
			args:     []string{"-Xpreprocessor", "-internal-isystem", "-Xpreprocessor", "relative/path"},
			flags:    cgoAbsEnvFlags,
			expected: []string{"-Xpreprocessor", "-internal-isystem", "-Xpreprocessor", "RELATIVE/PATH"},
		},
		{
			name:     "joined C++ path",
			args:     []string{"-stdlib++-isystemrelative/path"},
			flags:    cgoAbsEnvFlags,
			expected: []string{"-stdlib++-isystemRELATIVE/PATH"},
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

// Independent builds and path mapping can give actions identical arguments.
// Shared TMPDIR cleanup must not remove another action's scratch files.
func TestWorkDirIsolatedAcrossActions(t *testing.T) {
	t.Setenv("TMPDIR", t.TempDir())
	t.Setenv("RULES_GO_WORKDIR_ROOT", ".")
	root := t.TempDir()
	firstRoot := filepath.Join(root, "first-execroot")
	secondRoot := filepath.Join(root, "second-execroot")
	for _, dir := range []string{firstRoot, secondRoot} {
		if err := os.Mkdir(dir, 0o755); err != nil {
			t.Fatal(err)
		}
	}

	// os.Args stays identical, as it does for actions in independent builds.
	t.Chdir(firstRoot)
	first := &env{}
	firstDir, firstCleanup, err := first.workDir()
	if err != nil {
		t.Fatal(err)
	}
	defer firstCleanup()
	if !filepath.IsAbs(firstDir) || filepath.Dir(firstDir) != firstRoot {
		t.Fatalf("work directory must be absolute and inside the first execroot: %q", firstDir)
	}

	t.Chdir(secondRoot)
	second := &env{}
	secondDir, secondCleanup, err := second.workDir()
	if err != nil {
		t.Fatal(err)
	}
	defer secondCleanup()
	if !filepath.IsAbs(secondDir) || filepath.Dir(secondDir) != secondRoot {
		t.Fatalf("work directory must be absolute and inside the second execroot: %q", secondDir)
	}
	importcfg := filepath.Join(secondDir, "importcfg")
	if err := os.WriteFile(importcfg, []byte("packagefile example=example.a"), 0o600); err != nil {
		t.Fatal(err)
	}

	// Cleanup must still target the first execroot after changing directories.
	firstCleanup()
	if _, err := os.Stat(firstDir); !os.IsNotExist(err) {
		t.Fatalf("first action cleanup left its work directory behind: %v", err)
	}
	if _, err := os.Stat(importcfg); err != nil {
		t.Fatalf("first action cleanup removed second action's importcfg: %v", err)
	}
	if filepath.Base(firstDir) != filepath.Base(secondDir) {
		t.Fatalf("work directory name must stay stable across execroots: %q vs %q", firstDir, secondDir)
	}
}
