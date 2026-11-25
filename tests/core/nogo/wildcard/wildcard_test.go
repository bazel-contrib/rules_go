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

package wildcard_test

import (
	"strings"
	"testing"

	"github.com/bazelbuild/rules_go/go/tools/bazel_testing"
)

func TestMain(m *testing.M) {
	bazel_testing.TestMain(m, bazel_testing.Args{
		Main: `
-- BUILD.bazel --
load("@io_bazel_rules_go//go:def.bzl", "go_library", "nogo")

nogo(
    name = "my_nogo",
    vet = True,
    visibility = ["//visibility:public"],
)

go_library(
    name = "lib",
    srcs = ["lib.go"],
    importpath = "example.com/lib",
)

-- lib.go --
package lib
`,
		ModuleFileSuffix: `
go_sdk = use_extension("@io_bazel_rules_go//go:extensions.bzl", "go_sdk")
go_sdk.nogo(nogo = "//:my_nogo")
`,
	})
}

// TestWildcardsSkipNogo checks that both targets the nogo macro creates are
// tagged manual. If the alias were not, //... would match it and build nogo in
// the target configuration, a second build of nogo and all its analyzers
// alongside the one in the exec configuration that Go targets use.
func TestWildcardsSkipNogo(t *testing.T) {
	out, err := bazel_testing.BazelOutput("query", `attr(tags, "\bmanual\b", //:all)`)
	if err != nil {
		t.Fatal(err)
	}
	manual := strings.Fields(string(out))
	for _, want := range []string{"//:my_nogo", "//:my_nogo_actual"} {
		found := false
		for _, label := range manual {
			if label == want {
				found = true
			}
		}
		if !found {
			t.Errorf("%s is not tagged manual; manual targets: %v", want, manual)
		}
	}
}
