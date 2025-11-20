package wrapped_cgo_test

import (
	"testing"

	"github.com/bazelbuild/rules_go/go/tools/bazel_testing"
)

func TestMain(m *testing.M) {
	bazel_testing.TestMain(m, bazel_testing.Args{
		Main: `
-- wrapped_test.bzl --
CODE = """#!/usr/bin/env bash
echo "Running wrapped test {0}"
{0} $@
"""
def _wrapped_test_impl(ctx):
    out = ctx.actions.declare_file(ctx.label.name + ".sh")
    ctx.actions.write(out, CODE.format(ctx.executable.test.short_path))
    return [DefaultInfo(
        executable = out,
        default_runfiles = ctx.attr.test[DefaultInfo].default_runfiles.merge(ctx.runfiles(files=[out])),
    )]

wrapped_test = rule(
    implementation = _wrapped_test_impl,
    attrs = {
        "test": attr.label(mandatory = True, executable = True, cfg = "target"),
        # Required for Bazel to collect coverage of instrumented C/C++ binaries
        # executed by go_test.
        # This is just a shell script and thus cheap enough to depend on
        # unconditionally.
        "_collect_cc_coverage": attr.label(
            default = "@bazel_tools//tools/test:collect_cc_coverage",
            cfg = "exec",
        ),
        # Required for Bazel to merge coverage reports for Go and other
        # languages into a single report per test.
        # Using configuration_field ensures that the tool is only built when
        # run with bazel coverage, not with bazel test.
        "_lcov_merger": attr.label(
            default = configuration_field(fragment = "coverage", name = "output_generator"),
            cfg = "exec",
        ),
    },
    test = True,
)
-- BUILD.bazel --
load("@io_bazel_rules_go//go:def.bzl", "go_library", "go_test")
load("//:wrapped_test.bzl", "wrapped_test")

cc_library(
    name = "hello",
    srcs = ["hello.c"],
    hdrs = ["hello.h"],
    visibility = ["//visibility:public"],
)

cc_shared_library(
    name = "hello_shared_lib",
    deps = [":hello"],
)

# This weird dance is necessary to force rules_go to link against a shared library.
cc_library(
    name = "hello_shared",
    srcs = [":hello_shared_lib"],
    hdrs = ["hello.h"],
)

go_library(
    name = "go_hello",
    srcs = ["hello.go"],
    cgo = True,
    cdeps = [":hello_shared"],
    importpath = "example.com/hello",
    visibility = ["//visibility:public"],
)

go_test(
    name = "go_hello_test",
    srcs = ["hello_test.go"],
    deps = [":go_hello"],
    pure = "off",
    static = "off",
)

wrapped_test(
    name = "wrapped_go_hello_test",
    test = ":go_hello_test",
)
-- hello.h --
void hello();
-- hello.c --
#include "hello.h"
#include <stdio.h>
void hello() {
    printf("Hello, World!\n");
}
-- hello_test.go --
package hello_test
import (
    "testing"

    "example.com/hello"
)
func TestHello(t *testing.T) {
    hello.Hello()
}

-- hello.go --
package hello

// #include "hello.h"
import "C"
func Hello() {
    C.hello()
}
`,
	})
}

func TestWrappedCgo(t *testing.T) {
	if o, err := bazel_testing.BazelOutput("test", "//:wrapped_go_hello_test", "--test_env=GO_TEST_WRAP_TESTV=1", "--test_output=all", "--experimental_cc_shared_library"); err != nil {
		t.Fatalf("bazel test //:wrapped_go_hello_test: %v\n%s", err, string(o))
	} else {
		t.Logf("bazel test //:wrapped_go_hello_test: %s", string(o))
	}
}
