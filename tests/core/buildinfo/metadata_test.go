package main

import (
	"os/exec"
	"regexp"
	"strings"
	"testing"

	"github.com/bazelbuild/rules_go/go/tools/bazel"
)

const (
	noBuildInfoMarker = "NO_BUILD_INFO"
	expectedMainPath  = "github.com/bazelbuild/rules_go/tests/core/buildinfo"
)

func TestMetadata(t *testing.T) {
	// Run binary once and share output across subtests
	bin, ok := bazel.FindBinary("tests/core/buildinfo", "metadata_bin")
	if !ok {
		t.Fatal("could not find metadata_bin")
	}

	out, err := exec.Command(bin).CombinedOutput()
	if err != nil {
		t.Fatalf("failed to run metadata_bin: %v\noutput: %s", err, out)
	}

	output := string(out)
	t.Logf("metadata_bin output:\n%s", output)

	t.Run("BuildInfoPresent", func(t *testing.T) {
		if strings.Contains(output, noBuildInfoMarker) {
			t.Errorf("BuildInfo check failed: output contains %q marker, binary should have build info embedded\nGot output: %s", noBuildInfoMarker, output)
		}
	})

	t.Run("MainPackagePath", func(t *testing.T) {
		expectedPath := "Path=" + expectedMainPath
		if !strings.Contains(output, expectedPath) {
			t.Errorf("MainPackagePath check: output missing %q\nGot output: %s", expectedPath, output)
		}
	})

	t.Run("GoVersion", func(t *testing.T) {
		// Validate that Go version is present and well-formed (e.g., go1.20.1)
		goVersionPattern := regexp.MustCompile(`GoVersion=go\d+\.\d+`)
		if !goVersionPattern.MatchString(output) {
			t.Errorf("GoVersion check: output missing valid Go version pattern (expected GoVersion=go1.20.x format)\nGot output: %s", output)
		}
	})

	t.Run("TransitiveDependencies", func(t *testing.T) {
		// Check that all transitive Go dependencies are listed
		expectedDeps := []string{
			"github.com/bazelbuild/rules_go/tests/core/buildinfo/leaf",
			"github.com/bazelbuild/rules_go/tests/core/buildinfo/mid",
			"github.com/bazelbuild/rules_go/tests/core/buildinfo/top",
		}

		missingDeps := []string{}
		for _, dep := range expectedDeps {
			if !strings.Contains(output, "Dep="+dep) {
				missingDeps = append(missingDeps, dep)
			}
		}
		if len(missingDeps) > 0 {
			t.Errorf("TransitiveDependencies check: missing %d dependencies: %v\nGot output: %s",
				len(missingDeps), missingDeps, output)
		}
	})
}
