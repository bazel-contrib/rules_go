package filter_test_cases_test

import (
	"encoding/xml"
	"io/ioutil"
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
	name = "filter_test",
	srcs = ["filter_test.go"],
)
-- filter_test.go --
package filter_test_cases

import "testing"

func TestFoo(t *testing.T) {}
func TestTaskA(t *testing.T) {}
func TestTaskB(t *testing.T) {}
func TestTaskC(t *testing.T) {}
func TestTaskD(t *testing.T) {}
`,
	})
}

// xml test suites to check which test cases were run
type xmlTestCase struct {
	XMLName xml.Name `xml:"testcase"`
	Name    string   `xml:"name,attr"`
}
type xmlTestSuite struct {
	XMLName   xml.Name      `xml:"testsuite"`
	TestCases []xmlTestCase `xml:"testcase"`
}
type xmlTestSuites struct {
	XMLName xml.Name       `xml:"testsuites"`
	Suites  []xmlTestSuite `xml:"testsuite"`
}

func Test(t *testing.T) {
	tests := []struct {
		name                  string
		args                  []string
		expectedRunTestCases  map[string]bool
		expectedSkipTestCases map[string]struct{}
	}{
		{
			name:                  "skip_tests",
			args:                  []string{"test", "//:filter_test", "--test_env=GO_TEST_WRAP_TESTV=1", "--test_filter=-^TestFoo$,-^TestTaskA$"},
			expectedRunTestCases:  map[string]bool{"TestTaskB": false, "TestTaskC": false, "TestTaskD": false},
			expectedSkipTestCases: map[string]struct{}{"TestTaskA": {}, "TestFoo": {}},
		},
		{
			name:                  "run_only",
			args:                  []string{"test", "//:filter_test", "--test_env=GO_TEST_WRAP_TESTV=1", "--test_filter=^TestTask.+"},
			expectedRunTestCases:  map[string]bool{"TestTaskA": false, "TestTaskB": false, "TestTaskC": false, "TestTaskD": false},
			expectedSkipTestCases: map[string]struct{}{"TestFoo": {}},
		},
		{
			name:                  "filter_tests",
			args:                  []string{"test", "//:filter_test", "--test_env=GO_TEST_WRAP_TESTV=1", "--test_filter=^TestTask.+,-^TestTaskB$"},
			expectedRunTestCases:  map[string]bool{"TestTaskA": false, "TestTaskC": false, "TestTaskD": false},
			expectedSkipTestCases: map[string]struct{}{"TestTaskB": {}, "TestFoo": {}},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := bazel_testing.RunBazel(tt.args...); err != nil {
				t.Fatal(err)
			}
			p, err := bazel_testing.BazelOutput("info", "bazel-testlogs")
			if err != nil {
				t.Fatalf("could not find testlog root: %s", err)
			}
			path := filepath.Join(strings.TrimSpace(string(p)), "filter_test/test.xml")
			b, err := ioutil.ReadFile(path)
			if err != nil {
				t.Fatalf("could not read generated xml file: %s", err)
			}

			var suites xmlTestSuites
			if err := xml.Unmarshal(b, &suites); err != nil {
				t.Fatalf("could not unmarshall generated xml: %s", err)
			}

			for _, suite := range suites.Suites {
				for _, tc := range suite.TestCases {
					if _, ok := tt.expectedRunTestCases[tc.Name]; ok {
						tt.expectedRunTestCases[tc.Name] = true
					}
					if _, ok := tt.expectedSkipTestCases[tc.Name]; ok {
						t.Errorf("unexpected test case ran %s", tc.Name)
					}
				}
			}
			for testCase, found := range tt.expectedRunTestCases {
				if !found {
					t.Errorf("failed to run expected test case %s", testCase)
				}
			}
		})
	}
}
