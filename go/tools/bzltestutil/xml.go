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

package bzltestutil

import (
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io"
	"path"
	"sort"
	"strings"
	"time"
)

type xmlTestSuites struct {
	XMLName xml.Name       `xml:"testsuites"`
	Suites  []xmlTestSuite `xml:"testsuite"`
}

type xmlTestSuite struct {
	XMLName   xml.Name      `xml:"testsuite"`
	TestCases []xmlTestCase `xml:"testcase"`
	Errors    int           `xml:"errors,attr"`
	Failures  int           `xml:"failures,attr"`
	Skipped   int           `xml:"skipped,attr"`
	Tests     int           `xml:"tests,attr"`
	Time      string        `xml:"time,attr"`
	Name      string        `xml:"name,attr"`
	Timestamp string        `xml:"timestamp,attr,omitempty"`
}

type xmlTestCase struct {
	XMLName   xml.Name    `xml:"testcase"`
	Classname string      `xml:"classname,attr"`
	Name      string      `xml:"name,attr"`
	Time      string      `xml:"time,attr"`
	Failure   *xmlMessage `xml:"failure,omitempty"`
	Error     *xmlMessage `xml:"error,omitempty"`
	Skipped   *xmlMessage `xml:"skipped,omitempty"`
}

type xmlMessage struct {
	Message  string `xml:"message,attr"`
	Type     string `xml:"type,attr"`
	Contents string `xml:",chardata"`
}

// jsonEvent as encoded by the test2json package.
type jsonEvent struct {
	Time    *time.Time
	Action  string
	Package string
	Test    string
	Elapsed *float64
	Output  string
}

type testCase struct {
	state    string
	output   strings.Builder
	duration *float64
	timestamp *time.Time
}

const (
	timeoutPanicPrefix = "panic: test timed out after "
)

// json2xml converts test2json's output into an xml output readable by Bazel.
// http://windyroad.com.au/dl/Open%20Source/JUnit.xsd
func json2xml(r io.Reader, pkgName string) ([]byte, error) {
	testcases := make(map[string]*testCase)
	testCaseByName := func(name string) *testCase {
		if name == "" {
			return nil
		}
		if _, ok := testcases[name]; !ok {
			testcases[name] = &testCase{}
		}
		return testcases[name]
	}

	dec := json.NewDecoder(r)
	var inTimeoutSection, inRunningTestSection bool
	for {
		var e jsonEvent
		if err := dec.Decode(&e); err == io.EOF {
			break
		} else if err != nil {
			return nil, fmt.Errorf("error decoding test2json output: %s", err)
		}
		switch s := e.Action; s {
		case "run":
			if c := testCaseByName(e.Test); c != nil {
				c.state = s
				c.timestamp = e.Time
			}
		case "output":
			trimmedOutput := strings.TrimSpace(e.Output)
			if strings.HasPrefix(trimmedOutput, timeoutPanicPrefix) {
				inTimeoutSection = true
				continue
			}
			if inTimeoutSection && strings.HasPrefix(trimmedOutput, "running tests:") {
				inRunningTestSection = true
				continue
			}
			if inRunningTestSection {
				// looking for something like "TestReport/test_3 (2s)"
				parts := strings.Fields(e.Output)
				if len(parts) != 2 || !strings.HasPrefix(parts[1], "(") || !strings.HasSuffix(parts[1], ")") {
					inTimeoutSection = false
					inRunningTestSection = false
				} else if duration, err := time.ParseDuration(parts[1][1 : len(parts[1])-1]); err != nil {
					inTimeoutSection = false
					inRunningTestSection = false
				} else if c := testCaseByName(parts[0]); c != nil {
					c.state = "interrupt"
					seconds := duration.Seconds()
					c.duration = &seconds
					c.output.WriteString(e.Output)
				}
				continue
			}
			if c := testCaseByName(e.Test); c != nil {
				c.output.WriteString(e.Output)
			}
		case "skip":
			if c := testCaseByName(e.Test); c != nil {
				c.output.WriteString(e.Output)
				c.state = s
				c.duration = e.Elapsed
			}
		case "fail":
			if c := testCaseByName(e.Test); c != nil {
				c.state = s
				c.duration = e.Elapsed
			}
		case "pass":
			if c := testCaseByName(e.Test); c != nil {
				c.duration = e.Elapsed
				c.state = s
			}
		}
	}

	return xml.MarshalIndent(toXML(pkgName, testcases), "", "\t")
}

func toXML(pkgName string, testcases map[string]*testCase) *xmlTestSuites {
	cases := make([]string, 0, len(testcases))
	for k := range testcases {
		cases = append(cases, k)
	}
	sort.Strings(cases)

	suiteByName := make(map[string]*xmlTestSuite)
	var suiteNames []string

	for _, name := range cases {
		suiteName := strings.SplitN(name, "/", 2)[0]
		var suite *xmlTestSuite
		suite, ok := suiteByName[suiteName]
		if !ok {
			suite = &xmlTestSuite{
				Name: pkgName + "." + suiteName,
			}
			suiteByName[suiteName] = suite
			suiteNames = append(suiteNames, suiteName)
		}
		c := testcases[name]
		if name == suiteName {
			if c.duration != nil {
				suite.Time = fmt.Sprintf("%.3f", *c.duration)
			}
			if c.timestamp != nil {
				suite.Timestamp = c.timestamp.Format("2006-01-02T15:04:05.000Z")
			}
		}
		suite.Tests++
		newCase := xmlTestCase{
			Name:      name,
			Classname: path.Base(pkgName),
		}
		if c.duration != nil {
			newCase.Time = fmt.Sprintf("%.3f", *c.duration)
		}
		switch c.state {
		case "skip":
			suite.Skipped++
			newCase.Skipped = &xmlMessage{
				Message:  "Skipped",
				Contents: c.output.String(),
			}
		case "fail":
			suite.Failures++
			newCase.Failure = &xmlMessage{
				Message:  "Failed",
				Contents: c.output.String(),
			}
		case "interrupt":
			suite.Errors++
			newCase.Error = &xmlMessage{
				Message:  "Interrupted",
				Contents: c.output.String(),
			}
		case "pass":
			break
		default:
			suite.Errors++
			newCase.Error = &xmlMessage{
				Message:  "No pass/skip/fail event found for test",
				Contents: c.output.String(),
			}
		}
		suite.TestCases = append(suite.TestCases, newCase)
	}
	var suites xmlTestSuites
	// because test cases are sorted by name, the suite names are also sorted.
	for _, name := range suiteNames {
		suites.Suites = append(suites.Suites, *suiteByName[name])
	}
	return &suites
}
