package timeout_test

import (
	"os"
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
load("@io_bazel_rules_go//go:def.bzl", "go_test")

go_test(
	name = "timeout_test",
	srcs = ["timeout_test.go"],
)
-- timeout_test.go --
package timeout

import "testing"

func TestFoo(t *testing.T) {
	neverTerminates()
}

func neverTerminates() {
	for {}
}
`,
	})
}

func TestTimeout(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("stack traces on timeouts are not yet supported on Windows")
	}

	var stderr string
	if err := bazel_testing.RunBazel("test", "//:timeout_test", "--test_timeout=3", "--test_arg=-test.v"); err == nil {
		t.Fatal("expected bazel test to fail")
	} else if exitErr, ok := err.(*bazel_testing.StderrExitError); !ok || exitErr.Err.ExitCode() != 3 {
		t.Fatalf("expected bazel test to fail with exit code 3, got %v", err)
	} else {
		stderr = string(exitErr.Err.Stderr)
	}

	if !strings.Contains(stderr, "TIMEOUT: //:timeout_test") {
		t.Errorf("expect Bazel to report the test timed out: \n%s", stderr)
	}

	p, err := bazel_testing.BazelOutput("info", "bazel-testlogs")
	if err != nil {
		t.Fatalf("could not find testlogs root: %s", err)
	}
	path := filepath.Join(strings.TrimSpace(string(p)), "timeout_test/test.log")
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("could not read test log: %s", err)
	}

	testLog := string(b)
	if !strings.Contains(testLog, "panic: test timed out after 3s") {
		t.Errorf("test log does not contain expected header:\n%s", testLog)
	}
	if !strings.Contains(testLog, "timeout_test.neverTerminates(") {
		t.Errorf("test log does not contain expected stack trace:\n%s", testLog)
	}

	path = filepath.Join(strings.TrimSpace(string(p)), "timeout_test/test.xml")
	b, err = os.ReadFile(path)
	if err != nil {
		t.Fatalf("could not read test XML: %s", err)
	}

	testXML := string(b)
	if !strings.Contains(testXML, `<testcase classname="timeout_test" name="TestFoo"`) {
		t.Errorf("test XML does not contain expected element:\n%s", testXML)
	}
}
