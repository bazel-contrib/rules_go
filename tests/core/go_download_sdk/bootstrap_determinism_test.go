package go_download_sdk_test

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/bazelbuild/rules_go/go/tools/bazel_testing"
)

func TestMain(m *testing.M) {
	bazel_testing.TestMain(m, bazel_testing.Args{
		Main: `
-- BUILD.bazel --
exports_files(["go.mod"])

-- go.mod --
module bootstrap_determinism_test

go 1.26.0
`,
		ModuleFileSuffix: `
go_sdk = use_extension("@io_bazel_rules_go//go:extensions.bzl", "go_sdk")
go_sdk.from_file(
    name = "go_bootstrap_sdk",
    go_mod = "//:go.mod",
    experimental_build_compiler_from_source = True,
)
use_repo(go_sdk, "go_bootstrap_sdk")
`,
	})
}

func TestBootstrappedCompilerAndLinkerDeterministic(t *testing.T) {
	baseDir := mustMkdirTemp(t, "bootstrap_determinism_test_")
	t.Cleanup(func() {
		makeWritable(baseDir)
		_ = os.RemoveAll(baseDir)
	})
	first := buildBootstrapTools(t, filepath.Join(baseDir, "output-base-1"))
	second := buildBootstrapTools(t, filepath.Join(baseDir, "output-base-2"))

	assertSameBytes(t, "compile", first.compilePath, first.compileData, second.compilePath, second.compileData)
	assertSameBytes(t, "link", first.linkPath, first.linkData, second.linkPath, second.linkData)

	assertNoPathLeak(t, "compile", first.compileData, first.outputBase, second.outputBase)
	assertNoPathLeak(t, "link", first.linkData, first.outputBase, second.outputBase)
}

type bootstrapTools struct {
	outputBase  string
	compilePath string
	compileData []byte
	linkPath    string
	linkData    []byte
}

func buildBootstrapTools(t *testing.T, outputBase string) bootstrapTools {
	t.Helper()

	commonFlags := []string{
		"--noshow_progress",
		"--disk_cache=",
		"--remote_accept_cached=false",
		"--remote_upload_local_results=false",
	}

	target := "@go_bootstrap_sdk//:bootstrap_tools"
	runBazelWithOutputBase(t, outputBase, append([]string{"build", target}, commonFlags...)...)

	toolDir := findToolDir(t, outputBase, target, commonFlags)
	toolExt := ""
	if runtime.GOOS == "windows" {
		toolExt = ".exe"
	}

	compilePath := filepath.Join(toolDir, "compile"+toolExt)
	linkPath := filepath.Join(toolDir, "link"+toolExt)

	compileData, err := os.ReadFile(compilePath)
	if err != nil {
		t.Fatalf("reading %s: %v", compilePath, err)
	}
	linkData, err := os.ReadFile(linkPath)
	if err != nil {
		t.Fatalf("reading %s: %v", linkPath, err)
	}

	return bootstrapTools{
		outputBase:  outputBase,
		compilePath: compilePath,
		compileData: compileData,
		linkPath:    linkPath,
		linkData:    linkData,
	}
}

func findToolDir(t *testing.T, outputBase, target string, flags []string) string {
	t.Helper()

	out := bazelOutputWithOutputBase(t, outputBase, append([]string{"cquery", target, "--output=files"}, flags...)...)

	toolDirSuffix := path.Join("pkg", "tool", runtime.GOOS+"_"+runtime.GOARCH)
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("os.Getwd: %v", err)
	}
	for _, line := range strings.Split(string(out), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if strings.HasSuffix(line, toolDirSuffix) {
			return filepath.Join(wd, filepath.FromSlash(line))
		}
	}

	t.Fatalf("could not find %q in cquery output:\n%s", toolDirSuffix, out)
	return ""
}

func assertSameBytes(t *testing.T, toolName, firstPath string, first []byte, secondPath string, second []byte) {
	t.Helper()

	if bytes.Equal(first, second) {
		return
	}
	t.Fatalf(
		"%s differs across bootstrapped builds: %s (%s) vs %s (%s)",
		toolName,
		firstPath,
		sha256Hex(first),
		secondPath,
		sha256Hex(second),
	)
}

func assertNoPathLeak(t *testing.T, toolName string, data []byte, outputBases ...string) {
	t.Helper()

	checks := map[string]struct{}{
		"_bootstrap_sdk_workdir_": {},
	}
	for _, outputBase := range outputBases {
		if outputBase == "" {
			continue
		}
		checks[outputBase] = struct{}{}
		checks[filepath.ToSlash(outputBase)] = struct{}{}
	}
	for needle := range checks {
		if needle == "" {
			continue
		}
		if bytes.Contains(data, []byte(needle)) {
			t.Fatalf("%s contains non-hermetic path fragment %q", toolName, needle)
		}
	}
}

func sha256Hex(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func runBazelWithOutputBase(t *testing.T, outputBase string, args ...string) {
	t.Helper()

	cmd := bazel_testing.BazelCmd(args...)
	cmd.Args = append([]string{cmd.Args[0], "--output_base=" + filepath.ToSlash(outputBase)}, cmd.Args[1:]...)
	cmd.Env = withUTF8Locale(cmd.Env)
	stderr := &bytes.Buffer{}
	cmd.Stderr = io.MultiWriter(os.Stderr, stderr)
	if err := cmd.Run(); err != nil {
		t.Fatalf("bazel %s: %v\n%s", strings.Join(args, " "), err, stderr.String())
	}
}

func bazelOutputWithOutputBase(t *testing.T, outputBase string, args ...string) []byte {
	t.Helper()

	cmd := bazel_testing.BazelCmd(args...)
	cmd.Args = append([]string{cmd.Args[0], "--output_base=" + filepath.ToSlash(outputBase)}, cmd.Args[1:]...)
	cmd.Env = withUTF8Locale(cmd.Env)
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	cmd.Stdout = stdout
	cmd.Stderr = io.MultiWriter(os.Stderr, stderr)
	if err := cmd.Run(); err != nil {
		t.Fatalf("bazel %s: %v\n%s", strings.Join(args, " "), err, stderr.String())
	}
	return stdout.Bytes()
}

func withUTF8Locale(env []string) []string {
	out := make([]string, 0, len(env)+2)
	for _, v := range env {
		if strings.HasPrefix(v, "LANG=") || strings.HasPrefix(v, "LC_ALL=") || strings.HasPrefix(v, "LC_CTYPE=") {
			continue
		}
		out = append(out, v)
	}
	out = append(out, "LANG=C.UTF-8", "LC_ALL=C.UTF-8")
	return out
}

func mustMkdirTemp(t *testing.T, pattern string) string {
	t.Helper()

	tempRoot := os.Getenv("TEST_TMPDIR")
	dir, err := os.MkdirTemp(tempRoot, pattern)
	if err != nil {
		t.Fatalf("os.MkdirTemp: %v", err)
	}
	return dir
}

func makeWritable(root string) {
	_ = filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		info, statErr := d.Info()
		if statErr != nil {
			return nil
		}
		mode := info.Mode()
		if mode&0o200 == 0 {
			_ = os.Chmod(path, mode|0o200)
		}
		return nil
	})
}
