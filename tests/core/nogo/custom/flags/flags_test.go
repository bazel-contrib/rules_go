// Copyright 2019 The Bazel Authors. All rights reserved.
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

package flags_test

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"regexp"
	"testing"

	"github.com/bazelbuild/rules_go/go/tools/bazel_testing"
)

const origConfig = `# config = "",`

func TestMain(m *testing.M) {
	bazel_testing.TestMain(m, bazel_testing.Args{
		Nogo: "@//:nogo",
		Main: `
-- BUILD.bazel --
load("@io_bazel_rules_go//go:def.bzl", "go_library", "nogo")

nogo(
    name = "nogo",
    deps = [
        ":flagger",
    ],
    # config = "",
    visibility = ["//visibility:public"],
)

go_library(
    name = "flagger",
    srcs = ["flagger.go"],
    importpath = "flaggeranalyzer",
    deps = [
        "@org_golang_x_tools//go/analysis",
    ],
    visibility = ["//visibility:public"],
)

go_library(
    name = "some_file",
    srcs = ["some_file.go"],
    importpath = "somefile",
    deps = [":dep"],
)

go_library(
    name = "dep",
    srcs = ["dep.go"],
    importpath = "dep",
)

-- flagger.go --
// flagger crashes when three flags are set in the config or else it no-ops
package flagger

import (
	"errors"

	"golang.org/x/tools/go/analysis"
)

var (
	boolSwitch   			 bool
	stringSwitch 			 string
	intSwitch    			 int
	subanalyzerBoolSwitch    bool
)

var Subanalyzer = &analysis.Analyzer{
        Name: "flagger_subanalyzer",
        Run:  subanalyzerRun,
        Doc:  "Dummy subanalyzer that crashes if its flag is set'",
}

var Analyzer = &analysis.Analyzer{
        Name: "flagger",
        Run:  run,
        Doc:  "Dummy analyzer that crashes when all its flags are set correctly. It contains subanalyzer.",
	Requires: []*analysis.Analyzer{Subanalyzer},
}

func init() {
        Analyzer.Flags.BoolVar(&boolSwitch, "bool-switch", false, "Bool must be set to true to run")
        Analyzer.Flags.StringVar(&stringSwitch, "string-switch", "no", "String must be set to \"yes\" to run")
        Analyzer.Flags.IntVar(&intSwitch, "int-switch", 0, "Int must be set to 1 to run")
        Subanalyzer.Flags.BoolVar(&subanalyzerBoolSwitch, "subanalyzer-bool-switch", false, "Bool must be set to trigger subanalyzer")
}

func run(pass *analysis.Pass) (interface{}, error) {
        if !boolSwitch {
                return nil, nil
        }
        if stringSwitch != "yes" {
                return nil, nil
        }
        if intSwitch != 1 {
                return nil, nil
        }
        return nil, errors.New("all switches were set -> fail")
}

func subanalyzerRun(pass *analysis.Pass) (interface{}, error) {
	if !subanalyzerBoolSwitch {
		return nil, nil
	}
	return nil, errors.New("subanalyzer flag was set -> fail")
}

-- all_flags_set.json --
{
  "flagger": {
    "description": "this will crash on every file",
    "analyzer_flags": {
        "bool-switch": "true",
        "int-switch": "1",
        "string-switch": "yes"
    }
  }
}

-- two_flags_set.json --
{
  "flagger": {
    "description": "this will succeed on every file",
    "analyzer_flags": {
        "bool-switch": "true",
        "int-switch": "1"
    }
  }
}

-- subanalyzer_flag_set.json --
{
  "flagger_subanalyzer": {
    "description": "this will succeed on every file",
    "analyzer_flags": {
        "subanalyzer-bool-switch": "true"
    }
  }
}


-- invalid_int.json --
{
  "flagger": {
    "description": "this will crash immediately due to an invalid int flag",
    "analyzer_flags": {
        "int-switch": "one",
        "string-switch": "yes"
    }
  }
}

-- nonexistent_flag.json --
{
  "flagger": {
    "description": "this will crash immediately due to a nonexistent flag",
    "analyzer_flags": {
        "int-switch": "1",
        "bool-switch": "true",
        "string-switch": "yes",
        "description": "This is a good analyzer"
    }
  }
}

-- hyphenated_flag.json --
{
  "flagger": {
    "description": "this will crash immediately due to a hyphenated flag",
    "analyzer_flags": {
      "-int-switch": "1"
    }
  }
}

-- some_file.go --
// package somefile contains a file and has a dep
package somefile

import "dep"

func Baz() int {
	dep.D()
	return 1
}

-- dep.go --
package dep

func D() {
}

`,
	})
}

func Test(t *testing.T) {
	for _, test := range []struct {
		desc, config       string
		wantSuccess        bool
		includes, excludes []string
	}{
		{
			desc:        "config_flags_triggering_error",
			wantSuccess: false,
			config:      "all_flags_set.json",
			includes:    []string{"all switches were set -> fail"},
		}, {
			desc:        "config_subanalyzer_flags_triggering_error",
			wantSuccess: false,
			config:      "subanalyzer_flag_set.json",
			includes:    []string{"failed prerequisites: flagger_subanalyzer"},
		}, {
			desc:        "config_flags_triggering_success",
			wantSuccess: true,
			config:      "two_flags_set.json",
		}, {
			desc:        "invalid_int_triggering_error",
			wantSuccess: false,
			config:      "invalid_int.json",
			includes:    []string{"flagger: invalid value for flag: int-switch=one"},
		}, {
			desc:        "nonexistent_flag_triggering_error",
			wantSuccess: false,
			config:      "nonexistent_flag.json",
			includes:    []string{"flagger: unrecognized flag: description"},
		}, {
			desc:        "hyphenated_flag_triggering_error",
			wantSuccess: false,
			config:      "hyphenated_flag.json",
			includes:    []string{"flagger: flag should not begin with '-': -int-switch"},
		},
	} {
		t.Run(test.desc, func(t *testing.T) {
			if test.config != "" {
				customConfig := fmt.Sprintf("config = %q,", test.config)
				if err := replaceInFile("BUILD.bazel", origConfig, customConfig); err != nil {
					t.Fatal(err)
				}
				defer replaceInFile("BUILD.bazel", customConfig, origConfig)
			}

			cmd := bazel_testing.BazelCmd("build", "//:some_file")
			stderr := &bytes.Buffer{}
			cmd.Stderr = stderr
			if err := cmd.Run(); err == nil && !test.wantSuccess {
				t.Fatal("unexpected success")
			} else if err != nil && test.wantSuccess {
				t.Fatalf("unexpected error: %v", err)
			}

			for _, pattern := range test.includes {
				if matched, err := regexp.Match(pattern, stderr.Bytes()); err != nil {
					t.Fatal(err)
				} else if !matched {
					t.Errorf("got output:\n %s\n which does not contain pattern: %s", string(stderr.Bytes()), pattern)
				}
			}
			for _, pattern := range test.excludes {
				if matched, err := regexp.Match(pattern, stderr.Bytes()); err != nil {
					t.Fatal(err)
				} else if matched {
					t.Errorf("output contained pattern: %s", pattern)
				}
			}
		})
	}
}

func replaceInFile(path, old, new string) error {
	data, err := ioutil.ReadFile(path)
	if err != nil {
		return err
	}
	data = bytes.ReplaceAll(data, []byte(old), []byte(new))
	return ioutil.WriteFile(path, data, 0666)
}
