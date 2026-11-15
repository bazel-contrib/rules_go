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

package pathmapping_test

import (
	"encoding/json"
	"fmt"
	"regexp"
	"runtime"
	"slices"
	"strings"
	"testing"

	"github.com/bazelbuild/rules_go/go/tools/bazel_testing"
)

func TestMain(m *testing.M) {
	bazel_testing.TestMain(m, bazel_testing.Args{
		Nogo:         "@//:nogo",
		NogoIncludes: []string{"@//:__pkg__"},
		Main: `
-- BUILD.bazel --
load("@io_bazel_rules_go//go:def.bzl", "go_binary", "go_library", "nogo")
load(":generate.bzl", "generate")

go_library(
    name = "hello_lib",
    srcs = ["hello.go"],
    importpath = "example.com/hello",
)

go_binary(
    name = "hello",
    embed = [":hello_lib"],
)

nogo(
    name = "nogo",
    visibility = ["//visibility:public"],
    deps = ["//analyzer"],
)

[generate(
    name = name + "_source",
    out = name + "/generated.go",
    content = "package generated\n" + cgo_import + "func Generated() int { return " + value + " }\n",
) for name, cgo_import, value in [
    ("plain", "", "42"),
    ("cgo", "import \"C\"\n", "int(C.int(42))"),
]]

generate(
    name = "cgo_extra_source",
    out = "cgo_extra/generated.go",
    content = "package generated\nimport \"C\"\nfunc Extra() int { return int(C.int(24)) }\n",
)

[go_library(
    name = name + "_dep",
    srcs = [":" + name + "_source"] + ([":cgo_extra_source"] if name == "cgo" else []),
    cgo = name == "cgo",
    importpath = "example.com/" + name,
) for name in ["plain", "cgo"]]

[go_library(
    name = name + "_consumer",
    srcs = [name + "_consumer.go", "selected.go", "excluded.go"],
    deps = [":" + name + "_dep"],
    importpath = "example.com/" + name + "_consumer",
) for name in ["plain", "cgo"]]

-- generate.bzl --
def _generate_impl(ctx):
    ctx.actions.write(ctx.outputs.out, ctx.attr.content)
    return [DefaultInfo(files = depset([ctx.outputs.out]))]

generate = rule(
    implementation = _generate_impl,
    attrs = {"out": attr.output(mandatory = True), "content": attr.string()},
)

-- plain_consumer.go --
package consumer
import "example.com/plain"
var Value = selected(generated.Generated())

-- cgo_consumer.go --
package consumer
import "example.com/cgo"
var Value = selected(generated.Generated())

-- selected.go --
//go:build mappingtest

package consumer
func selected(value int) int { return value }

-- excluded.go --
//go:build !mappingtest

package consumer
func selected(value string) int { return len(value) }

-- analyzer/BUILD.bazel --
load("@io_bazel_rules_go//go:def.bzl", "go_library")
go_library(
    name = "analyzer",
    srcs = ["analyzer.go"],
    importpath = "example.com/analyzer",
    visibility = ["//visibility:public"],
    deps = ["@org_golang_x_tools//go/analysis"],
)

-- analyzer/analyzer.go --
package analyzer

import (
    "fmt"
    "go/ast"
    "os"
    "strings"

    "golang.org/x/tools/go/analysis"
)

var Analyzer = &analysis.Analyzer{
    Name: "mappedpositions",
    Doc: "checks generated source positions and reports imported positions",
    Run: run,
}

func run(pass *analysis.Pass) (any, error) {
    for _, file := range pass.Files {
        for _, decl := range file.Decls {
            fn, ok := decl.(*ast.FuncDecl)
            if !ok || (fn.Name.Name != "Generated" && fn.Name.Name != "Extra") { continue }
            // For cgo this position comes from a generated //line directive,
            // not directly from a path-mapped command-line argument.
            pos := pass.Fset.Position(fn.Pos())
            content, err := os.ReadFile(pos.Filename)
            if err != nil {
                return nil, fmt.Errorf("cannot read generated source at %s: %w", pos, err)
            }
            if !strings.Contains(string(content), "func " + fn.Name.Name + "(") {
                return nil, fmt.Errorf("source at %s does not contain %s", pos, fn.Name.Name)
            }
        }
    }
    for ident, obj := range pass.TypesInfo.Uses {
        if obj.Name() == "Generated" && obj.Pkg() != pass.Pkg {
            // This position was serialized into the dependency's export data.
            pos := pass.Fset.Position(obj.Pos())
            pass.Reportf(ident.Pos(), "imported Generated from %s", pos)
        }
    }
    return nil, nil
}

-- hello.go --
package hello

import "fmt"

func Hello() { fmt.Println("hello") }
`,
	})
}

// unstrippedConfigSegment matches a path segment right after "bazel-out/"
// that carries configuration information (e.g. "darwin_arm64-fastbuild",
// "k8-opt-exec-ST-<hash>"). Path-mapped arguments replace this segment with
// the fixed placeholder "cfg", so a match here means the arg was not path
// mapped.
var unstrippedConfigSegment = regexp.MustCompile(`bazel-out/[^/]*-(fastbuild|dbg|opt)[^/]*/`)

func TestSdkArgIsPathMapped(t *testing.T) {
	cases := []struct {
		mnemonic string
		target   string
	}{
		{"GoCompilePkg", "//:hello_lib"},
		{"GoInfo", "@io_bazel_rules_go//:go_info"},
	}
	for _, c := range cases {
		t.Run(c.mnemonic, func(t *testing.T) {
			stripped := aqueryArgs(t, c.mnemonic, c.target, "--experimental_output_paths=strip")
			assertPathMapped(t, stripped, "-sdk")
			// GoInfo does not pass -goroot; only assert it for actions that do.
			if _, ok := flagValue(stripped, "-goroot"); ok {
				assertPathMapped(t, stripped, "-goroot")
				// Sanity check: without path mapping, -goroot points to a
				// bazel-out path with a real config segment. This makes sure
				// the regex used by assertPathMapped would actually catch a
				// regression if -goroot (or -sdk) stopped being path mapped.
				unstripped := aqueryArgs(t, c.mnemonic, c.target)
				assertMatchesConfigSegment(t, unstripped, "-goroot")
			}
		})
	}
}

// TestNogoPathMapping executes the actions as well as inspecting their command
// lines: mapped arguments alone cannot verify positions embedded in export data
// or in cgo-generated //line directives.
func TestNogoPathMapping(t *testing.T) {
	flags := []string{
		"--experimental_output_paths=strip",
		"--@io_bazel_rules_go//go/config:tags=mappingtest",
		"--spawn_strategy=sandboxed",
	}
	for _, name := range []string{"plain", "cgo"} {
		t.Run(name, func(t *testing.T) {
			if runtime.GOOS == "windows" {
				t.Skip("sandboxed execution is unavailable on Windows")
			}
			target := "//:" + name + "_consumer"
			args := aqueryArgs(t, "RunNogo", target, flags...)
			for _, flag := range []string{"-arc", "-stdlib_export", "-out_facts", "-out", "-nogo"} {
				assertPathMapped(t, args, flag)
			}
			depArgs := aqueryArgs(t, "RunNogo", "//:"+name+"_dep", flags...)
			if name == "cgo" {
				// Cgo output still embeds unmapped paths from compilation, so
				// its analysis action conservatively uses the same paths.
				assertMatchesConfigSegment(t, depArgs, "-src")
				assertMatchesConfigSegment(t, depArgs, "-ignore_src")
			} else {
				assertPathMapped(t, depArgs, "-src")
			}
			build := append([]string{"build"}, flags...)
			cmd := bazel_testing.BazelCmd(append(build, target)...)
			out, err := cmd.CombinedOutput()
			if err == nil {
				t.Fatalf("expected analyzer diagnostic, build succeeded:\n%s", out)
			}
			diagnostic := regexp.MustCompile(name + `_consumer.go:3:[0-9]+: imported Generated from (.*` + name + `/generated.go:[0-9]+(?::[0-9]+)?)`)
			match := diagnostic.FindStringSubmatch(string(out))
			if match == nil {
				t.Fatalf("missing imported source-position diagnostic:\n%s", out)
			}
			if name == "cgo" {
				if !unstrippedConfigSegment.MatchString(match[1]) {
					t.Errorf("expected original cgo source position, got %s", match[1])
				}
			} else if unstrippedConfigSegment.MatchString(match[1]) || !strings.Contains(match[1], "bazel-out/cfg/") {
				t.Errorf("imported source position was not path mapped: %s", match[1])
			}
		})
	}
}

// aqueryArgs runs bazel aquery --output=jsonproto and returns the arguments
// of the (single) matching action.
func aqueryArgs(t *testing.T, mnemonic, target string, extraFlags ...string) []string {
	t.Helper()
	cmd := []string{
		"aquery",
		"--include_commandline",
		"--include_param_files",
		"--output=jsonproto",
	}
	cmd = append(cmd, extraFlags...)
	cmd = append(cmd, fmt.Sprintf(`mnemonic("%s", %s)`, mnemonic, target))
	out, err := bazel_testing.BazelOutput(cmd...)
	if err != nil {
		t.Fatalf("bazel aquery failed: %v", err)
	}
	var parsed struct {
		Actions []struct {
			Arguments []string `json:"arguments"`
		} `json:"actions"`
	}
	if err := json.Unmarshal(out, &parsed); err != nil {
		t.Fatalf("failed to decode aquery output: %v\n%s", err, out)
	}
	if len(parsed.Actions) != 1 {
		t.Fatalf("expected 1 %s action for %s, got %d", mnemonic, target, len(parsed.Actions))
	}
	return parsed.Actions[0].Arguments
}

// assertPathMapped verifies that the value following the given flag does not
// contain a bazel-out path with an unstripped configuration segment. Values
// that come through path mapping are rewritten to use the fixed "bazel-out/cfg/"
// prefix. Non-bazel-out values (source paths under external/, etc.) are left
// alone by both the mapping and this check.
func assertPathMapped(t *testing.T, args []string, flag string) {
	t.Helper()
	val, ok := flagValue(args, flag)
	if !ok {
		t.Fatalf("flag %q not found in args %v", flag, args)
	}
	if unstrippedConfigSegment.MatchString(val) {
		t.Fatalf("flag %q value %q contains an unstripped config segment; "+
			"the argument must be produced via args.add_all(..., map_each = _dirname) "+
			"so that path mapping applies", flag, val)
	}
}

// assertMatchesConfigSegment verifies the flag's value is a bazel-out path
// with a config segment. It's used to sanity-check that
// unstrippedConfigSegment matches real Bazel output, so that a broken regex
// doesn't turn assertPathMapped into a no-op.
func assertMatchesConfigSegment(t *testing.T, args []string, flag string) {
	t.Helper()
	val, ok := flagValue(args, flag)
	if !ok {
		t.Fatalf("flag %q not found in args %v", flag, args)
	}
	if !unstrippedConfigSegment.MatchString(val) {
		t.Fatalf("expected flag %q value %q to contain a config segment "+
			"(without --experimental_output_paths=strip); the regex may be "+
			"out of date", flag, val)
	}
}

// flagValue returns the argument that follows flag, or false if flag is not
// present.
func flagValue(args []string, flag string) (string, bool) {
	i := slices.Index(args, flag)
	if i < 0 || i+1 >= len(args) {
		return "", false
	}
	return args[i+1], true
}
