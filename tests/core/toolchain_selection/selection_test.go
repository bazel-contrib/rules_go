package toolchain_selection_test

import (
	_ "embed"
	"strings"
	"testing"

	"github.com/bazelbuild/rules_go/go/tools/bazel_testing"
)

//go:embed selection_test.bzl
var selectionRules string

func TestMain(m *testing.M) {
	bazel_testing.TestMain(m, bazel_testing.Args{
		SkipHostGoSDK: true,
		Main: `
-- BUILD.bazel --
load(":selection_test.bzl", "alternate_sdk_test", "default_sdk_test", "sdk_probe")

exports_files(["go.mod"])
sdk_probe(name = "probe")
default_sdk_test(name = "default_sdk_test", expected_version = "1.23.5", target_under_test = ":probe")
alternate_sdk_test(name = "alternate_sdk_test", expected_version = "1.24.0", target_under_test = ":probe")
-- go.mod --
module sdk_selection

go 1.23.5
-- selection_test.bzl --
` + selectionRules,
		ModuleFileSuffix: `
bazel_dep(name = "bazel_skylib", version = "1.8.1")

go_sdk = use_extension("@io_bazel_rules_go//go:extensions.bzl", "go_sdk")
go_sdk.from_file(name = "repository_only_from_file", go_mod = "//:go.mod", register_toolchains = False)
go_sdk.download(name = "repository_only_download", version = "1.23.5", register_toolchains = False)
go_sdk.download(name = "selected_sdk", version = "1.23.5")
go_sdk.download(name = "alternate_sdk", version = "1.24.0")
use_repo(go_sdk, "go_toolchains", "repository_only_download", "repository_only_from_file", "selected_sdk", "alternate_sdk")
`,
	})
}

func TestSDKSelection(t *testing.T) {
	if err := bazel_testing.RunBazel("test", "//:default_sdk_test", "//:alternate_sdk_test"); err != nil {
		t.Fatal(err)
	}
}

func TestRepositoryOnlySDKsRemainAddressable(t *testing.T) {
	if err := bazel_testing.RunBazel("build", "@repository_only_download//:ROOT", "@repository_only_from_file//:ROOT"); err != nil {
		t.Fatal(err)
	}
}

func TestRepositoryOnlySDKsAreNotRegistered(t *testing.T) {
	output, err := bazel_testing.BazelOutput("query", "@go_toolchains//:all")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(output), "repository_only_") {
		t.Fatalf("repository-only SDK entered the toolchain catalogue:\n%s", output)
	}
}
