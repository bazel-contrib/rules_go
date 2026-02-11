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

package from_go_work_file_test

import (
	"io/ioutil"
	"testing"

	"github.com/bazelbuild/rules_go/go/tools/bazel_testing"
)

func TestMain(m *testing.M) {
	bazel_testing.TestMain(m, bazel_testing.Args{
		Main: `
-- BUILD.bazel --
load("@io_bazel_rules_go//go:def.bzl", "go_test")

go_test(
    name = "version_test",
    srcs = ["version_test.go"],
)

-- version_test.go --
package version_test

import (
    "fmt"
    "flag"
    "runtime"
    "testing"
)

var want = flag.String("version", "", "")

func Test(t *testing.T) {
    fmt.Println(runtime.Version())
    if v := runtime.Version(); v != *want {
        t.Errorf("got version %q; want %q", v, *want)
    }
}
`,
		ModuleFileSuffix: `
go_sdk = use_extension("@io_bazel_rules_go//go:extensions.bzl", "go_sdk")
go_sdk.from_file(name = "sdk_under_test", go_work = "//:go.work")
`,
	})
}

func Test(t *testing.T) {
	for _, test := range []struct {
		desc, go_work, want string
	}{
		{
			desc: "toolchain",
			go_work: `
go 1.23.0

toolchain go1.24.0

use (
    .  // unused, just here to test the go.work parser
)
`,
			want: "go1.24.0",
		},
		{
			desc: "toolchain minor version",
			go_work: `
go 1.23.0

toolchain go1.24.1

use (
    .  // unused, just here to test the go.work parser
)
`,
			want: "go1.24.1",
		},
		{
			desc: "go only",
			go_work: `
go 1.16

use (
    .  // unused, just here to test the go.work parser
)
`,
			want: "go1.16",
		},
		{
			desc: "missing go",
			go_work: `
use (
    .  // unused, just here to test the go.work parser
)
`,
			want: "go1.18",
		},
	} {
		t.Run(test.desc, func(t *testing.T) {
			if err := ioutil.WriteFile("go.work", []byte(test.go_work), 0o666); err != nil {
				t.Fatal(err)
			}
			args := []string{
				"test",
				"--test_arg=-version=" + test.want,
				// This next flag forces the test environment to choose its own
				// module's SDK, because `bazel_testing` uses `go_wrap_sdk` to
				// create its own SDK in the WORKSPACE file.
				"--@io_bazel_rules_go//go/toolchain:sdk_name=sdk_under_test",
				"//:version_test",
			}
			if err := bazel_testing.RunBazel(args...); err != nil {
				t.Fatal(err)
			}
		})
	}
}
