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

package race_test

import (
	"bytes"
	"errors"
	"fmt"
	"os/exec"
	"runtime"
	"strings"
	"testing"

	"github.com/bazelbuild/rules_go/go/tools/bazel_testing"
)

func TestMain(m *testing.M) {
	bazel_testing.TestMain(m, bazel_testing.Args{
		Main: `
-- BUILD.bazel --
load("@io_bazel_rules_go//go:def.bzl", "go_binary", "go_library", "go_test")

go_library(
    name = "racy",
    srcs = [
        "race_off.go",
        "race_on.go",
        "racy.go",
        "empty.s", # verify #2143
    ],
    importpath = "example.com/racy",
)

go_binary(
    name = "racy_cmd",
    srcs = ["main.go"],
    embed = [":racy"],
)

go_binary(
    name = "racy_cmd_race_mode",
    srcs = ["main.go"],
    embed = [":racy"],
    race = "on",
)

go_test(
    name = "racy_test",
    srcs = ["racy_test.go"],
    embed = [":racy"],
)

go_test(
    name = "racy_test_race_mode",
    srcs = ["racy_test.go"],
    embed = [":racy"],
    race = "on",
)

go_binary(
    name = "pure_bin",
    srcs = ["pure_bin.go"],
    pure = "on",
)

go_binary(
    name = "pure_race_bin",
    srcs = ["pure_bin.go"],
    pure = "on",
    race = "on",
)

go_library(
		name = "coverrace",
		srcs = ["coverrace.go"],
		importpath = "example.com/coverrace",
)

go_test(
		name = "coverrace_test",
		srcs = ["coverrace_test.go"],
		embed = [":coverrace"],
    race = "on",
)

go_test(
    name = "timeout_test",
    srcs = ["timeout_test.go"],
    race = "on",
)
-- race_off.go --
// +build !race

package main

const RaceEnabled = false

-- race_on.go --
// +build race

package main

const RaceEnabled = true

-- racy.go --
package main

import (
	"flag"
	"fmt"
	"os"
)

var wantRace = flag.Bool("wantrace", false, "")

func Race() {
	if *wantRace != RaceEnabled {
		fmt.Fprintf(os.Stderr, "!!! -wantrace is %v, but RaceEnabled is %v\n", *wantRace, RaceEnabled)
		os.Exit(1)
	}

	done := make(chan bool)
	m := make(map[string]string)
	m["name"] = "world"
	go func() {
		m["name"] = "data race"
		done <- true
	}()
	fmt.Println("Hello,", m["name"])
	<-done
}

-- main.go --
package main

import "flag"

func main() {
	flag.Parse()
	Race()
}

-- racy_test.go --
package main

import "testing"

func TestRace(t *testing.T) {
	Race()
}

-- empty.s --
-- pure_bin.go --
// +build !race

// pure_bin will not build in race mode, since its sources will be excluded.
package main

func main() {}

-- coverrace.go --
package coverrace
// copied from https://hermanschaaf.com/running-the-go-race-detector-with-cover/
func add100() int {
	total := 0
	c := make(chan int, 1)
	for i := 0; i < 100; i++ {
		go func(chan int) {
			c <- 1
		}(c)
	}
	for u := 0; u < 100; u++ {
		total += <-c
	}
	return total
}

-- coverrace_test.go --
package coverrace
// copied from https://hermanschaaf.com/running-the-go-race-detector-with-cover/

import "testing"

func TestCoverRace(t *testing.T) {
	got := add100()
	if got != 100 {
		t.Errorf("got %d, want %d", got, 100)
	}
}

-- timeout_test.go --
package main

import (
	"testing"
	"time"
)

func TestTimeout(t *testing.T) {
	time.Sleep(10*time.Second)
}
`,
	})
}

func TestTimeout(t *testing.T) {
	cmd := bazel_testing.BazelCmd("test", "//:timeout_test", "--test_timeout=1", "--test_output=all")
	stdout := &bytes.Buffer{}
	cmd.Stdout = stdout
	t.Logf("running: %s", strings.Join(cmd.Args, " "))
	err := cmd.Run()
	t.Log(stdout.String())
	if err == nil {
		t.Fatalf("expected bazel test to fail")
	}
	var xerr *exec.ExitError
	if !errors.As(err, &xerr) || xerr.ExitCode() != bazel_testing.TESTS_FAILED {
		t.Fatalf("unexpected error: %v", err)
	}
	if bytes.Contains(stdout.Bytes(), []byte("WARNING: DATA RACE")) {
		t.Fatalf("unexpected data race; command failed with: %v\nstdout:\n%s", err, stdout.Bytes())
	}
}

func Test(t *testing.T) {
	for _, test := range []struct {
		desc, cmd, target                    string
		featureFlag, wantRace, wantBuildFail bool
	}{
		{
			desc:   "cmd_auto",
			cmd:    "run",
			target: "//:racy_cmd",
		}, {
			desc:     "cmd_attr",
			cmd:      "run",
			target:   "//:racy_cmd_race_mode",
			wantRace: true,
		}, {
			desc:        "cmd_feature",
			cmd:         "run",
			target:      "//:racy_cmd",
			featureFlag: true,
			wantRace:    true,
		}, {
			desc:   "test_auto",
			cmd:    "test",
			target: "//:racy_test",
		}, {
			desc:     "test_attr",
			cmd:      "test",
			target:   "//:racy_test_race_mode",
			wantRace: true,
		}, {
			desc:        "test_feature",
			cmd:         "test",
			target:      "//:racy_test",
			featureFlag: true,
			wantRace:    true,
		}, {
			desc:        "pure_bin",
			cmd:         "build",
			target:      "//:pure_bin",
			featureFlag: true,
		}, {
			desc:          "pure_race_bin",
			cmd:           "build",
			target:        "//:pure_race_bin",
			wantBuildFail: true,
		}, {
			desc:        "cover_race",
			cmd:         "coverage",
			target:      "//:coverrace_test",
			featureFlag: true,
		},
	} {
		t.Run(test.desc, func(t *testing.T) {
			// TODO(#2518): fix coverage tests on Windows
			if test.cmd == "coverage" && runtime.GOOS == "windows" {
				t.Skip("TODO(#2518): fix and enable coverage tests on Windows")
			}
			args := []string{test.cmd}
			if test.featureFlag {
				args = append(args, "--@io_bazel_rules_go//go/config:race")
			}
			args = append(args, test.target)
			if test.cmd == "test" {
				args = append(args, fmt.Sprintf("--test_arg=-wantrace=%v", test.wantRace))
			} else if test.cmd == "run" {
				args = append(args, "--", fmt.Sprintf("-wantrace=%v", test.wantRace))
			}
			cmd := bazel_testing.BazelCmd(args...)
			stderr := &bytes.Buffer{}
			cmd.Stderr = stderr
			t.Logf("running: bazel %s", strings.Join(args, " "))
			if err := cmd.Run(); err != nil {
				var xerr *exec.ExitError
				if !errors.As(err, &xerr) {
					t.Fatalf("unexpected error: %v", err)
				}
				if xerr.ExitCode() == bazel_testing.BUILD_FAILURE {
					if !test.wantBuildFail {
						t.Fatalf("unexpected build failure: %v\nstderr:\n%s", err, stderr.Bytes())
					}
					return
				} else if xerr.ExitCode() == bazel_testing.TESTS_FAILED {
					if bytes.Contains(stderr.Bytes(), []byte("!!!")) {
						t.Fatalf("error running %s:\n%s", strings.Join(cmd.Args, " "), stderr.Bytes())
					} else if !test.wantRace {
						t.Fatalf("error running %s without race enabled\n%s", strings.Join(cmd.Args, " "), stderr.Bytes())
					}
				} else if test.wantRace {
					if !bytes.Contains(stderr.Bytes(), []byte("WARNING: DATA RACE")) {
						t.Fatalf("wanted data race; command failed with: %v\nstderr:\n%s", err, stderr.Bytes())
					}
					return
				} else {
					t.Fatalf("unexpected error: %v\nstderr:\n%s", err, stderr.Bytes())
				}
			} else if test.wantRace {
				t.Fatalf("command %s with race enabled did not fail", strings.Join(cmd.Args, " "))
			} else if test.wantBuildFail {
				t.Fatalf("target %s did not fail to build", test.target)
			}
		})
	}
}
