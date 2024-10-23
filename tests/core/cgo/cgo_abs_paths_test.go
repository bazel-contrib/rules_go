package cgo_abs_paths_test

import (
	"strings"
	"testing"

	"github.com/bazelbuild/rules_go/go/tools/bazel_testing"
)

func TestMain(m *testing.M) {
	bazel_testing.TestMain(m, bazel_testing.Args{
		Main: `
-- main.go --
package main

/*
#include <stdio.h>

void hello() {
	printf("Hello, world!\n");
}
*/
import "C"

func main() {
	C.hello()
}
-- BUILD.bazel --
load("@io_bazel_rules_go//go:def.bzl", "go_test", "go_library")

go_library(
    name = "example_lib",
    srcs = ["main.go"],
	cgo = True,
    importpath = "example.com/cmd/example",
)

go_test(
    name = "example",
    embed = [":example_lib"],
	pure = "off",
)
`,
		SetUp: func() error {
			// we have to force a bazel clean because this test needs to
			// fully execute every time so that we can actually scan the output
			return bazel_testing.RunBazel("clean")
		},
	})
}

func TestCgoAbsolutePaths(t *testing.T) {
	_, stderrBytes, err := bazel_testing.BazelOutputWithInput(nil,
		"build",
		"--linkopt=-Lan/imaginary/lib",
		"--linkopt=-v",
		"--copt=-Ian/imaginary/include",
		"--copt=-v",
		"-s",
		"//:example",
	)
	if err != nil {
		t.Fatal(err)
	}

	stderr := string(stderrBytes)

	if strings.Contains(stderr, "__GO_BAZEL_CC_PLACEHOLDER__") {
		t.Log(stderr)
		t.Fatal("Found absolute path placeholder string in go linker command output")
	}

	if !strings.Contains(stderr, "-Lan/imaginary/lib") {
		t.Log(stderr)
		t.Fatal("Could not find --linkopt in go linker command output")
	}

	if !strings.Contains(stderr, "-Ian/imaginary/include") {
		t.Log(stderr)
		t.Fatal("Could not find --copt in go compiler command output")
	}
}
