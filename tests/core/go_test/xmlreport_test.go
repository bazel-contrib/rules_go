// Copyright 2020 The Bazel Authors. All rights reserved.
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

package test_filter_test

import (
	"encoding/xml"
	"io/ioutil"
	"path/filepath"
	"reflect"
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
    name = "xml_test",
    importpath = "github.com/bazelbuild/rules_go/tests/core/go_test/xml_test",
    srcs = ["xml_test.go"],
)

-- xml_test.go --
package test

import (
    "math/rand"
    "testing"
    "time"
)

func TestPass(t *testing.T) {
    t.Parallel()
}

func TestPassLog(t *testing.T) {
    t.Parallel()
    t.Log("pass")
}

func TestFail(t *testing.T) {
    t.Error("Not working")
}

func TestSubtests(t *testing.T) {
    for i, subtest := range []string{"subtest a", "testB", "another subtest"} {
        t.Run(subtest, func(t *testing.T) {
            t.Logf("from subtest %s", subtest)
            time.Sleep(time.Duration(rand.Intn(10)) * time.Millisecond)
            t.Logf("from subtest %s", subtest)
            if i%3 == 0 {
                t.Skip("skipping this test")
            }
            if i%2 == 0 {
                t.Fail()
            }
        })
    }
}
`,
	})
}

// test execution time attributes will vary per testrun, so we must parse the
// xml to inspect a subset of testresults
type xmlTestSuite struct {
	XMLName  xml.Name `xml:"testsuite"`
	Errors   int      `xml:"errors,attr"`
	Failures int      `xml:"failures,attr"`
	Skipped  int      `xml:"skipped,attr"`
	Tests    int      `xml:"tests,attr"`
	Name     string   `xml:"name,attr"`
}
type xmlTestSuites struct {
	XMLName xml.Name       `xml:"testsuites"`
	Suites  []xmlTestSuite `xml:"testsuite"`
}

func Test(t *testing.T) {
	tests := []struct {
		name     string
		args     []string
		expected xmlTestSuites
	}{
		{
			name: "default",
			args: []string{"test", "//:xml_test"},
			expected: xmlTestSuites{
				XMLName: xml.Name{Local: "testsuites"},
				Suites: []xmlTestSuite{
					{
						XMLName:  xml.Name{Local: "testsuite"},
						Name:     "github.com/bazelbuild/rules_go/tests/core/go_test/xml_test.TestFail",
						Errors:   0,
						Failures: 1,
						Skipped:  0,
						Tests:    1,
					},
					{
						XMLName:  xml.Name{Local: "testsuite"},
						Name:     "github.com/bazelbuild/rules_go/tests/core/go_test/xml_test.TestSubtests",
						Errors:   0,
						Failures: 2,
						Skipped:  0,
						Tests:    2,
					},
				},
			},
		},
		{
			name: "verbose",
			args: []string{"test", "--test_env=GO_TEST_WRAP_TESTV=1", "//:xml_test"},
			expected: xmlTestSuites{
				XMLName: xml.Name{Local: "testsuites"},
				Suites: []xmlTestSuite{
					{
						XMLName:  xml.Name{Local: "testsuite"},
						Name:     "github.com/bazelbuild/rules_go/tests/core/go_test/xml_test.TestFail",
						Errors:   0,
						Failures: 1,
						Skipped:  0,
						Tests:    1,
					},
					{
						XMLName:  xml.Name{Local: "testsuite"},
						Name:     "github.com/bazelbuild/rules_go/tests/core/go_test/xml_test.TestPass",
						Errors:   0,
						Failures: 0,
						Skipped:  0,
						Tests:    1,
					},
					{
						XMLName:  xml.Name{Local: "testsuite"},
						Name:     "github.com/bazelbuild/rules_go/tests/core/go_test/xml_test.TestPassLog",
						Errors:   0,
						Failures: 0,
						Skipped:  0,
						Tests:    1,
					},
					{
						XMLName:  xml.Name{Local: "testsuite"},
						Name:     "github.com/bazelbuild/rules_go/tests/core/go_test/xml_test.TestSubtests",
						Errors:   0,
						Failures: 2,
						Skipped:  1,
						Tests:    4,
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := bazel_testing.RunBazel(tt.args...)
			if err == nil {
				t.Fatal("expected bazel test to have failed")
			}
			if xerr, ok := err.(*bazel_testing.StderrExitError); !ok || xerr.Err.ExitCode() != 3 {
				t.Fatalf("expected bazel tests to fail with exit code 3 (TESTS_FAILED), got: %s", err)
			}

			p, err := bazel_testing.BazelOutput("info", "bazel-testlogs")
			if err != nil {
				t.Fatalf("could not find testlog root: %s", err)
			}
			path := filepath.Join(strings.TrimSpace(string(p)), "xml_test/test.xml")
			b, err := ioutil.ReadFile(path)
			if err != nil {
				t.Fatalf("could not read generated xml file: %s", err)
			}

			var suites xmlTestSuites
			if err := xml.Unmarshal(b, &suites); err != nil {
				t.Fatalf("could not unmarshall generated xml: %s", err)
			}

			if !reflect.DeepEqual(suites, tt.expected) {
				t.Fatalf("expected %#v, got: %#v", tt.expected, suites)
			}
		})
	}
}
