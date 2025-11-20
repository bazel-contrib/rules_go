package xml_panic_test

import (
	"encoding/xml"
	"errors"
	"io/ioutil"
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
	name = "panic_test",
	srcs = ["panic_test.go"],
)
-- panic_test.go --
package panic_test

import "testing"

func TestPanic(t *testing.T) {
	panic("panic by design")
}
`,
	})
}

type testError struct {
	Message string `xml:"message,attr"`
}

type xmlTestCase struct {
	XMLName xml.Name   `xml:"testcase"`
	Name    string     `xml:"name,attr"`
	Time    string     `xml:"time,attr"`
	Error   *testError `xml:"error"`
}
type xmlTestSuite struct {
	XMLName   xml.Name      `xml:"testsuite"`
	Name    string     `xml:"name,attr"`
	TestCases []xmlTestCase `xml:"testcase"`
	Time      string         `xml:"time,attr"`
}
type xmlTestSuites struct {
	XMLName xml.Name       `xml:"testsuites"`
	Suites  []xmlTestSuite `xml:"testsuite"`
}

func Test(t *testing.T) {
	err := bazel_testing.RunBazel("test", "//:panic_test", "--test_env=GO_TEST_WRAP_TESTV=1")
	if err == nil {
		t.Fatal("expected test to fail")
	}
	var exErr *exec.ExitError
	if !errors.As(err, &exErr) {
		t.Fatalf("unexpected error: %v", err)
	} else if exErr.ExitCode() != 3 {
		t.Fatalf("unexpected exit code %d\n%s", exErr.ExitCode(), exErr.Stderr)
	}
	p, err := bazel_testing.BazelOutput("info", "bazel-testlogs")
	if err != nil {
		t.Fatalf("could not find testlog root: %s", err)
	}
	path := filepath.Join(strings.TrimSpace(string(p)), "panic_test/test.xml")
	b, err := ioutil.ReadFile(path)
	if err != nil {
		t.Fatalf("could not read generated xml file: %s", err)
	}

	var suites xmlTestSuites
	if err := xml.Unmarshal(b, &suites); err != nil {
		t.Fatalf("could not unmarshall generated xml: %s", err)
	}

	for _, suite := range suites.Suites {
		if suite.Time == "" {
			t.Errorf("empty time attribute for test suite %q", suite.Name)
		}
		for _, tc := range suite.TestCases {
			if tc.Time == "" {
				t.Errorf("empty time attribute for test case %q", tc.Name)
			}
			if tc.Error != nil {
				t.Errorf("unexpected error for test case %q: %s", tc.Name, tc.Error.Message)
			}
		}
	}
}
