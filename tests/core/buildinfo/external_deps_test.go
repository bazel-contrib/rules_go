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
	externalDepModule = "golang.org/x/sys"
)

func TestExternalDeps(t *testing.T) {
	// Run binary once and share output across subtests
	bin, ok := bazel.FindBinary("tests/core/buildinfo", "external_deps_bin")
	if !ok {
		t.Fatal("could not find external_deps_bin")
	}

	out, err := exec.Command(bin).CombinedOutput()
	if err != nil {
		t.Fatalf("failed to run external_deps_bin: %v\noutput: %s", err, out)
	}

	output := string(out)
	t.Logf("external_deps_bin output:\n%s", output)

	t.Run("BuildInfoPresent", func(t *testing.T) {
		if strings.Contains(output, noBuildInfoMarker) {
			t.Errorf("BuildInfo check failed: output contains %q marker, binary should have build info embedded", noBuildInfoMarker)
		}
	})

	t.Run("ExternalDependencyListed", func(t *testing.T) {
		// Check that the external dependency is listed
		expectedDep := "Dep=" + externalDepModule
		if !strings.Contains(output, expectedDep) {
			t.Errorf("ExternalDependencyListed check: output missing %q\nGot output: %s", expectedDep, output)
		}
	})

	t.Run("ExternalDependencyVersion", func(t *testing.T) {
		// Version should be set (not (devel) for external dependencies)
		// This validates that the aspect collected package_metadata correctly
		lines := strings.Split(output, "\n")
		foundSysDep := false
		versionPattern := regexp.MustCompile(`^v\d+\.\d+\.\d+`)

		for _, line := range lines {
			if strings.HasPrefix(line, "Dep="+externalDepModule+"@") {
				foundSysDep = true
				version := strings.TrimPrefix(line, "Dep="+externalDepModule+"@")

				if version == "(devel)" {
					t.Errorf("ExternalDependencyVersion check for %s: got version=(devel), want real version (format: v0.0.0)\nFull line: %q", externalDepModule, line)
				} else if !versionPattern.MatchString(version) {
					t.Errorf("ExternalDependencyVersion check for %s: got malformed version=%q, want format v0.0.0\nFull line: %q", externalDepModule, version, line)
				} else {
					t.Logf("Found %s with valid version: %s", externalDepModule, version)
				}
			}
		}

		if !foundSysDep {
			t.Errorf("ExternalDependencyVersion check: %s dependency not found in output\nSearched for pattern: Dep=%s@<version>\nGot output: %s",
				externalDepModule, externalDepModule, output)
		}
	})
}
