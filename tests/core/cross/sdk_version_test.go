// Copyright 2022 The Bazel Authors. All rights reserved.
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

package go_download_sdk_test

import (
	"fmt"
	"strings"
	"testing"
	"text/template"

	"github.com/bazelbuild/rules_go/go/tools/bazel_testing"
)

type testcase struct {
	Name, SDKVersion, expectedVersion string
}

var testCases = []testcase{
	{
		Name:            "major_version",
		SDKVersion:      "1",
		expectedVersion: "go1.20",
	},
	{
		Name:            "minor_version",
		SDKVersion:      "1.20",
		expectedVersion: "go1.20",
	},
	{
		Name:            "patch_version",
		SDKVersion:      "1.20.0",
		expectedVersion: "go1.20",
	},
	{
		Name:            "1_21_minor_version",
		SDKVersion:      "1.21",
		expectedVersion: "go1.21.0",
	},
	{
		Name:            "1_21_patch_version",
		SDKVersion:      "1.21.1",
		expectedVersion: "go1.21.1",
	},
	{
		Name:            "1_21_release_candidate",
		SDKVersion:      "1.21rc2",
		expectedVersion: "go1.21rc2",
	},
}

func TestMain(m *testing.M) {
	mainFilesTmpl := template.Must(template.New("").Parse(`
-- main.go --
package main

import (
  "fmt"
	"runtime"
)

func main() {
  fmt.Print(runtime.Version())
}
-- BUILD.bazel --
load("@io_bazel_rules_go//go:def.bzl", "go_binary", "go_cross_binary")

go_binary(
  name = "print_version",
  srcs = ["main.go"],
)
{{range .TestCases}}
go_cross_binary(
  name = "{{.Name}}",
  target = ":print_version",
  sdk_version = "{{.SDKVersion}}",
)
{{end}}
`))
	tmplValues := struct {
		TestCases []testcase
	}{
		TestCases: testCases,
	}
	mainFilesBuilder := new(strings.Builder)
	if err := mainFilesTmpl.Execute(mainFilesBuilder, tmplValues); err != nil {
		panic(err)
	}

	bazel_testing.TestMain(m, bazel_testing.Args{
		Main: mainFilesBuilder.String(),
		// The test declares its own @go_sdk.
		SkipHostGoSDK: true,
		ModuleFileSuffix: `
go_sdk = use_extension("@io_bazel_rules_go//go:extensions.bzl", "go_sdk")
go_sdk.download(
    name = "go_sdk",
    version = "1.20",
)
go_sdk.download(
    name = "go_sdk_1_21_0",
    version = "1.21.0",
)
go_sdk.download(
    name = "go_sdk_1_21_1",
    version = "1.21.1",
)
go_sdk.download(
    name = "go_sdk_1_21_rc2",
    version = "1.21rc2",
)
`,
	})
}

func Test(t *testing.T) {
	for _, test := range testCases {
		t.Run(test.Name, func(t *testing.T) {
			output, err := bazel_testing.BazelOutput("run", fmt.Sprintf("//:%s", test.Name))
			if err != nil {
				t.Fatal(err)
			}
			actualVersion := string(output)
			if actualVersion != test.expectedVersion {
				t.Fatal("actual", actualVersion, "vs expected", test.expectedVersion)
			}
		})
	}
}
