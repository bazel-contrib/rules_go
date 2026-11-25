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

package json_report_test

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
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
    name = "json_test",
    importpath = "github.com/bazelbuild/rules_go/tests/core/go_test/json_test",
    srcs = ["json_test.go"],
)

go_test(
    name = "panic_test",
    importpath = "github.com/bazelbuild/rules_go/tests/core/go_test/panic_test",
    srcs = ["panic_test.go"],
)

-- json_test.go --
package test

import "testing"

func TestPass(t *testing.T) {}

func TestPassLog(t *testing.T) {
    t.Log("pass")
}

func TestFail(t *testing.T) {
    t.Error("Not working")
}

-- panic_test.go --
package test

import "testing"

func TestPanic(t *testing.T) {
    panic("panic by design")
}
`,
	})
}

type event struct {
	Action  string
	Package string
	Test    string
	Output  string
}

func runTests(t *testing.T, args ...string) {
	t.Helper()
	err := bazel_testing.RunBazel(append([]string{"test", "--nozip_undeclared_test_outputs"}, args...)...)
	if err == nil {
		t.Fatal("expected bazel test to have failed")
	}
	var exErr *exec.ExitError
	if !errors.As(err, &exErr) || exErr.ExitCode() != 3 {
		t.Fatalf("expected bazel test to fail with exit code 3 (TESTS_FAILED), got: %v", err)
	}
}

func jsonReportPath(t *testing.T, target string) string {
	t.Helper()
	out, err := bazel_testing.BazelOutput("info", "bazel-testlogs")
	if err != nil {
		t.Fatalf("could not find testlog root: %s", err)
	}
	return filepath.Join(strings.TrimSpace(string(out)), target, "test.outputs", "test2json.json")
}

func readEvents(t *testing.T, target string) []event {
	t.Helper()
	b, err := os.ReadFile(jsonReportPath(t, target))
	if err != nil {
		t.Fatalf("could not read generated json file: %s", err)
	}
	var events []event
	scanner := bufio.NewScanner(bytes.NewReader(b))
	for scanner.Scan() {
		var e event
		if err := json.Unmarshal(scanner.Bytes(), &e); err != nil {
			t.Fatalf("could not unmarshal event %q: %s", scanner.Text(), err)
		}
		events = append(events, e)
	}
	if err := scanner.Err(); err != nil {
		t.Fatalf("could not scan generated json file: %s", err)
	}
	if len(events) == 0 {
		t.Fatal("generated json file has no events")
	}
	return events
}

func hasEvent(events []event, action, test string) bool {
	for _, e := range events {
		if e.Action == action && e.Test == test {
			return true
		}
	}
	return false
}

func TestDisabled(t *testing.T) {
	runTests(t, "//:json_test")
	if _, err := os.Stat(jsonReportPath(t, "json_test")); !os.IsNotExist(err) {
		t.Fatalf("expected no json report without GO_TEST_WRAP_JSON, got: %v", err)
	}
}

func TestEnabled(t *testing.T) {
	runTests(t, "--test_env=GO_TEST_WRAP_JSON=1", "--test_env=GO_TEST_WRAP_TESTV=1", "//:json_test")
	events := readEvents(t, "json_test")

	for _, e := range events {
		if e.Package != "github.com/bazelbuild/rules_go/tests/core/go_test/json_test" {
			t.Errorf("unexpected package in event %+v", e)
		}
	}
	for _, want := range []struct{ action, test string }{
		{"pass", "TestPass"},
		{"pass", "TestPassLog"},
		{"fail", "TestFail"},
		{"fail", ""},
	} {
		if !hasEvent(events, want.action, want.test) {
			t.Errorf("missing %q event for test %q", want.action, want.test)
		}
	}
	found := false
	for _, e := range events {
		if e.Action == "output" && e.Test == "TestPassLog" && strings.Contains(e.Output, "pass") {
			found = true
		}
	}
	if !found {
		t.Error("missing output event for TestPassLog")
	}
}

func TestPanic(t *testing.T) {
	runTests(t, "--test_env=GO_TEST_WRAP_JSON=1", "--test_env=GO_TEST_WRAP_TESTV=1", "//:panic_test")
	events := readEvents(t, "panic_test")

	if !hasEvent(events, "fail", "TestPanic") {
		t.Error("missing fail event for TestPanic")
	}
	if last := events[len(events)-1]; last.Action != "fail" || last.Test != "" {
		t.Errorf("expected a package-level fail as the last event, got %+v", last)
	}
}
