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

package go_download_sdk_test

import (
	"bytes"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
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

go_test(
    name = "patch_test",
    srcs = ["patch_test.go"],
)

-- version_test.go --
package version_test

import (
	"flag"
	"runtime"
	"testing"
)

var want = flag.String("version", "", "")

func Test(t *testing.T) {
	if v := runtime.Version(); v != *want {
		t.Errorf("got version %q; want %q", v, *want)
	}
}
-- patch_test.go --
package version_test

import (
	"os"
	"testing"
)

func Test(t *testing.T) {
	if v := os.SayHello; v != "Hello"{
		t.Errorf("got version %q; want \"Hello\"", v)
	}
}
`,
	})
}

func Test(t *testing.T) {
	for _, test := range []struct {
		desc, rule       string
		optToWantVersion map[string]string
		fetchOnly        string
	}{
		{
			desc: "version",
			rule: `
load("@io_bazel_rules_go//go:deps.bzl", "go_download_sdk")

go_download_sdk(
    name = "go_sdk",
    version = "1.18",
)

`,
			optToWantVersion: map[string]string{"": "go1.18"},
		},
		{
			desc: "custom_archives",
			rule: `
load("@io_bazel_rules_go//go:deps.bzl", "go_download_sdk")

go_download_sdk(
    name = "go_sdk",
    sdks = {
        "darwin_amd64": ("go1.18.darwin-amd64.tar.gz", "70bb4a066997535e346c8bfa3e0dfe250d61100b17ccc5676274642447834969"),
        "darwin_arm64": ("go1.18.darwin-arm64.tar.gz", "9cab6123af9ffade905525d79fc9ee76651e716c85f1f215872b5f2976782480"),
        "linux_amd64": ("go1.18.linux-amd64.tar.gz", "e85278e98f57cdb150fe8409e6e5df5343ecb13cebf03a5d5ff12bd55a80264f"),
        "windows_amd64": ("go1.18.windows-amd64.zip", "65c5c0c709a7ca1b357091b10b795b439d8b50e579d3893edab4c7e9b384f435"),
    },
)
`,
			optToWantVersion: map[string]string{"": "go1.18"},
		},
		{
			desc: "multiple_sdks",
			rule: `
load("@io_bazel_rules_go//go:deps.bzl", "go_download_sdk", "go_host_sdk")

go_download_sdk(
    name = "go_sdk",
    version = "1.18",
)
go_download_sdk(
    name = "go_sdk_1_19",
    version = "1.19",
)
go_download_sdk(
    name = "go_sdk_1_19_1",
    version = "1.19.1",
)
`,
			optToWantVersion: map[string]string{
				"": "go1.18",
				"--@io_bazel_rules_go//go/toolchain:sdk_version=remote": "go1.18",
				"--@io_bazel_rules_go//go/toolchain:sdk_version=1":      "go1.18",
				"--@io_bazel_rules_go//go/toolchain:sdk_version=1.19":   "go1.19",
				"--@io_bazel_rules_go//go/toolchain:sdk_version=1.19.0": "go1.19",
				"--@io_bazel_rules_go//go/toolchain:sdk_version=1.19.1": "go1.19.1",
			},
		},
		{
			// Cover workaround for #2771.
			desc: "windows_zip",
			rule: `
load("@io_bazel_rules_go//go:deps.bzl", "go_download_sdk")

go_download_sdk(
    name = "go_sdk",
	goarch = "amd64",
	goos = "windows",
	version = "1.20.4",
)
`,
			fetchOnly: "@go_sdk//:BUILD.bazel",
		},
		{
			desc: "multiple_sdks_by_name",
			rule: `
load("@io_bazel_rules_go//go:deps.bzl", "go_download_sdk", "go_host_sdk")

go_download_sdk(
    name = "go_sdk",
    version = "1.23.5",
)
go_download_sdk(
    name = "go_sdk_1_18",
    version = "1.18",
)
go_download_sdk(
    name = "go_sdk_1_18_1",
    version = "1.18.1",
)
go_download_sdk(
    name = "go_sdk_with_experiments",
    version = "1.23.5",
	experiments = ["rangefunc"],
)
`,
			optToWantVersion: map[string]string{
				"": "go1.23.5",
				"--@io_bazel_rules_go//go/toolchain:sdk_name=go_sdk_1_18_1":           "go1.18.1",
				"--@io_bazel_rules_go//go/toolchain:sdk_name=go_sdk_1_18":             "go1.18",
				"--@io_bazel_rules_go//go/toolchain:sdk_name=go_sdk":                  "go1.23.5",
				"--@io_bazel_rules_go//go/toolchain:sdk_name=go_sdk_with_experiments": "go1.23.5 X:rangefunc",
			},
		},
	} {
		t.Run(test.desc, func(t *testing.T) {
			origWorkspaceData, err := ioutil.ReadFile("WORKSPACE")
			if err != nil {
				t.Fatal(err)
			}

			i := bytes.Index(origWorkspaceData, []byte("go_rules_dependencies()"))
			if i < 0 {
				t.Fatal("could not find call to go_rules_dependencies()")
			}

			buf := &bytes.Buffer{}
			buf.Write(origWorkspaceData[:i])
			buf.WriteString(test.rule)
			buf.WriteString(`
go_rules_dependencies()

go_register_toolchains()
`)
			if err := ioutil.WriteFile("WORKSPACE", buf.Bytes(), 0666); err != nil {
				t.Fatal(err)
			}
			defer func() {
				if err := ioutil.WriteFile("WORKSPACE", origWorkspaceData, 0666); err != nil {
					t.Errorf("error restoring WORKSPACE: %v", err)
				}
			}()

			if test.fetchOnly != "" {
				if err := bazel_testing.RunBazel("fetch", test.fetchOnly); err != nil {
					t.Fatal(err)
				}
				return
			}

			for opt, wantVersion := range test.optToWantVersion {
				t.Run(wantVersion, func(t *testing.T) {
					args := []string{
						"test",
						"//:version_test",
						"--test_arg=-version=" + wantVersion,
					}
					if opt != "" {
						args = append(args, opt)
					}
					if err := bazel_testing.RunBazel(args...); err != nil {
						t.Fatal(err)
					}
				})
			}
		})
	}
}

func TestPatch(t *testing.T) {
	origWorkspaceData, err := ioutil.ReadFile("WORKSPACE")
	if err != nil {
		t.Fatal(err)
	}

	i := bytes.Index(origWorkspaceData, []byte("go_rules_dependencies()"))
	if i < 0 {
		t.Fatal("could not find call to go_rules_dependencies()")
	}

	buf := &bytes.Buffer{}
	buf.Write(origWorkspaceData[:i])
	buf.WriteString(`
load("@io_bazel_rules_go//go:deps.bzl", "go_download_sdk")

go_download_sdk(
    name = "go_sdk_patched",
	version = "1.21.1",
    patch_strip = 1,
    patches = ["//:test.patch"],
)

go_rules_dependencies()

go_register_toolchains()
`)
	if err := ioutil.WriteFile("WORKSPACE", buf.Bytes(), 0666); err != nil {
		t.Fatal(err)
	}

	patchContent := []byte(`diff --git a/src/os/dir.go b/src/os/dir.go
index 5306bcb..d110a19 100644
--- a/src/os/dir.go
+++ b/src/os/dir.go
@@ -17,6 +17,8 @@ const (
 	readdirFileInfo
 )

+const SayHello = "Hello"
+
 // Readdir reads the contents of the directory associated with file and
 // returns a slice of up to n FileInfo values, as would be returned
 // by Lstat, in directory order. Subsequent calls on the same file will yield
`)

	if err := ioutil.WriteFile("test.patch", patchContent, 0666); err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := ioutil.WriteFile("WORKSPACE", origWorkspaceData, 0666); err != nil {
			t.Errorf("error restoring WORKSPACE: %v", err)
		}
	}()

	if err := bazel_testing.RunBazel(
		"test",
		"//:patch_test",
	); err != nil {
		t.Fatal(err)
	}
}

func TestExperimentalBuildCompilerFromSourceDoesNotRequireToolchainBuildSetting(t *testing.T) {
	for _, test := range []struct {
		name, bootstrapAttr string
	}{
		{
			name:          "experimental_build_compiler_from_source",
			bootstrapAttr: "experimental_build_compiler_from_source = True,",
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			origWorkspaceData, err := ioutil.ReadFile("WORKSPACE")
			if err != nil {
				t.Fatal(err)
			}

			i := bytes.Index(origWorkspaceData, []byte("go_rules_dependencies()"))
			if i < 0 {
				t.Fatal("could not find call to go_rules_dependencies()")
			}

			buf := &bytes.Buffer{}
			buf.Write(origWorkspaceData[:i])
			buf.WriteString(`
load("@io_bazel_rules_go//go:deps.bzl", "go_download_sdk")

go_download_sdk(
    name = "go_sdk",
    version = "1.26.0",
    ` + test.bootstrapAttr + `
)

go_rules_dependencies()

go_register_toolchains()
`)
			if err := ioutil.WriteFile("WORKSPACE", buf.Bytes(), 0666); err != nil {
				t.Fatal(err)
			}
			defer func() {
				if err := ioutil.WriteFile("WORKSPACE", origWorkspaceData, 0666); err != nil {
					t.Errorf("error restoring WORKSPACE: %v", err)
				}
			}()

			if err := bazel_testing.RunBazel("query", "@go_sdk_toolchains//:all"); err != nil {
				t.Fatal(err)
			}

			outputBase, err := bazel_testing.BazelOutput("info", "output_base")
			if err != nil {
				t.Fatal(err)
			}

			toolchainsBuildFile := filepath.Join(strings.TrimSpace(string(outputBase)), "external", "go_sdk_toolchains", "BUILD.bazel")
			toolchainsBuildData, err := os.ReadFile(toolchainsBuildFile)
			if err != nil {
				t.Fatalf("reading %s: %v", toolchainsBuildFile, err)
			}

			if bytes.Contains(toolchainsBuildData, []byte(`sdk_source = `)) {
				t.Fatalf("go_sdk_toolchains should not require an sdk_source build setting when go_download_sdk(%s):\n%s", test.bootstrapAttr, toolchainsBuildData)
			}
		})
	}
}

func TestExperimentalBootstrapWithRulesShellToolchains(t *testing.T) {
	origWorkspaceData, err := ioutil.ReadFile("WORKSPACE")
	if err != nil {
		t.Fatal(err)
	}

	i := bytes.Index(origWorkspaceData, []byte("go_rules_dependencies()"))
	if i < 0 {
		t.Fatal("could not find call to go_rules_dependencies()")
	}

	buf := &bytes.Buffer{}
	buf.Write(origWorkspaceData[:i])
	buf.WriteString(`
load("@io_bazel_rules_go//go:deps.bzl", "go_download_sdk")

go_download_sdk(
    name = "go_sdk",
    version = "1.26.0",
    experimental_build_compiler_from_source = True,
)

go_rules_dependencies()

load("@rules_shell//shell:repositories.bzl", "rules_shell_toolchains")

rules_shell_toolchains()

go_register_toolchains()
`)
	if err := ioutil.WriteFile("WORKSPACE", buf.Bytes(), 0666); err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := ioutil.WriteFile("WORKSPACE", origWorkspaceData, 0666); err != nil {
			t.Errorf("error restoring WORKSPACE: %v", err)
		}
	}()

	if err := bazel_testing.RunBazel(
		"cquery",
		"//:version_test",
	); err != nil {
		t.Fatal(err)
	}
}

func TestExperimentalBootstrapHostCompatibleSDKRoot(t *testing.T) {
	origWorkspaceData, err := ioutil.ReadFile("WORKSPACE")
	if err != nil {
		t.Fatal(err)
	}

	i := bytes.Index(origWorkspaceData, []byte("go_rules_dependencies()"))
	if i < 0 {
		t.Fatal("could not find call to go_rules_dependencies()")
	}

	buf := &bytes.Buffer{}
	buf.Write(origWorkspaceData[:i])
	buf.WriteString(`
load("@io_bazel_rules_go//go:deps.bzl", "go_download_sdk")

go_download_sdk(
    name = "go_sdk",
    version = "1.26.0",
    experimental_build_compiler_from_source = True,
)

go_rules_dependencies()

go_register_toolchains()
`)
	if err := ioutil.WriteFile("WORKSPACE", buf.Bytes(), 0666); err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := ioutil.WriteFile("WORKSPACE", origWorkspaceData, 0666); err != nil {
			t.Errorf("error restoring WORKSPACE: %v", err)
		}
	}()

	if err := bazel_testing.RunBazel(
		"query",
		"set(@go_host_compatible_sdk_label//:all @go_sdk//:host_compatible_root_file)",
	); err != nil {
		t.Fatal(err)
	}

	outputBase, err := bazel_testing.BazelOutput("info", "output_base")
	if err != nil {
		t.Fatal(err)
	}
	outputBasePath := strings.TrimSpace(string(outputBase))

	defsPath := filepath.Join(outputBasePath, "external", "go_host_compatible_sdk_label", "defs.bzl")
	defsData, err := os.ReadFile(defsPath)
	if err != nil {
		t.Fatalf("reading %s: %v", defsPath, err)
	}
	if !bytes.Contains(defsData, []byte(`HOST_COMPATIBLE_SDK = Label("@go_sdk//:host_compatible_root_file")`)) {
		t.Fatalf("go_host_compatible_sdk_label should point to :host_compatible_root_file:\n%s", defsData)
	}

	sdkBuildPath := filepath.Join(outputBasePath, "external", "go_sdk", "BUILD.bazel")
	sdkBuildData, err := os.ReadFile(sdkBuildPath)
	if err != nil {
		t.Fatalf("reading %s: %v", sdkBuildPath, err)
	}
	if !bytes.Contains(sdkBuildData, []byte(`name = "host_compatible_root_file"`)) {
		t.Fatalf("go_sdk BUILD.bazel should define :host_compatible_root_file:\n%s", sdkBuildData)
	}
	if !bytes.Contains(sdkBuildData, []byte(`srcs = [":bootstrap_root_file"]`)) {
		t.Fatalf("go_sdk :host_compatible_root_file should point to :bootstrap_root_file in bootstrap mode:\n%s", sdkBuildData)
	}
	if !bytes.Contains(sdkBuildData, []byte(`exec_compatible_with = [`)) {
		t.Fatalf("go_sdk bootstrap rule should constrain its execution platform to the SDK platform:\n%s", sdkBuildData)
	}
	if !bytes.Contains(sdkBuildData, []byte(`name = "go_sdk_srcs"`)) {
		t.Fatalf("go_sdk BUILD.bazel should define :go_sdk_srcs in bootstrap mode:\n%s", sdkBuildData)
	}
	if !bytes.Contains(sdkBuildData, []byte(`srcs = [":bootstrap_srcs"]`)) {
		t.Fatalf("go_sdk :go_sdk_srcs should point to :bootstrap_srcs in bootstrap mode:\n%s", sdkBuildData)
	}
	if !bytes.Contains(sdkBuildData, []byte(`go_sdk_srcs = [":go_sdk_srcs"]`)) {
		t.Fatalf("go_sdk should wire go_sdk_srcs to :go_sdk_srcs in bootstrap mode:\n%s", sdkBuildData)
	}
	if !bytes.Contains(sdkBuildData, []byte(`package_list_srcs = [":srcs"]`)) {
		t.Fatalf("go_sdk :package_list should be derived from filtered :srcs in bootstrap mode:\n%s", sdkBuildData)
	}
}

func TestCustomGoSDKHostCompatibleLabelBackwardCompatibility(t *testing.T) {
	origWorkspaceData, err := ioutil.ReadFile("WORKSPACE")
	if err != nil {
		t.Fatal(err)
	}

	i := bytes.Index(origWorkspaceData, []byte("go_rules_dependencies()"))
	if i < 0 {
		t.Fatal("could not find call to go_rules_dependencies()")
	}

	customSDKPath, err := filepath.Abs("custom_go_sdk")
	if err != nil {
		t.Fatalf("resolving custom SDK path: %v", err)
	}
	if err := os.MkdirAll(customSDKPath, 0777); err != nil {
		t.Fatalf("creating custom SDK path: %v", err)
	}
	defer func() {
		if err := os.RemoveAll(customSDKPath); err != nil {
			t.Errorf("removing custom SDK path: %v", err)
		}
	}()
	if err := os.WriteFile(filepath.Join(customSDKPath, "ROOT"), nil, 0666); err != nil {
		t.Fatalf("creating custom SDK ROOT file: %v", err)
	}

	buf := &bytes.Buffer{}
	buf.Write(origWorkspaceData[:i])
	buf.WriteString(`
load("@bazel_tools//tools/build_defs/repo:local.bzl", "new_local_repository")

new_local_repository(
    name = "go_sdk",
    path = "`)
	buf.WriteString(filepath.ToSlash(customSDKPath))
	buf.WriteString(`",
    build_file_content = """package(default_visibility = ["//visibility:public"])
exports_files(["ROOT"])
""",
)

go_rules_dependencies()
`)
	if err := ioutil.WriteFile("WORKSPACE", buf.Bytes(), 0666); err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := ioutil.WriteFile("WORKSPACE", origWorkspaceData, 0666); err != nil {
			t.Errorf("error restoring WORKSPACE: %v", err)
		}
	}()

	if err := bazel_testing.RunBazel(
		"query",
		"set(@go_host_compatible_sdk_label//:all @go_sdk//:ROOT)",
	); err != nil {
		t.Fatal(err)
	}

	outputBase, err := bazel_testing.BazelOutput("info", "output_base")
	if err != nil {
		t.Fatal(err)
	}
	outputBasePath := strings.TrimSpace(string(outputBase))

	defsPath := filepath.Join(outputBasePath, "external", "go_host_compatible_sdk_label", "defs.bzl")
	defsData, err := os.ReadFile(defsPath)
	if err != nil {
		t.Fatalf("reading %s: %v", defsPath, err)
	}
	if !bytes.Contains(defsData, []byte(`HOST_COMPATIBLE_SDK = Label("@go_sdk//:ROOT")`)) {
		t.Fatalf("go_host_compatible_sdk_label should point to :ROOT for custom go_sdk repos:\n%s", defsData)
	}
}

func TestExperimentalBootstrapWithBzlmod(t *testing.T) {
	origModuleData, err := ioutil.ReadFile("MODULE.bazel")
	hadModule := err == nil
	if err != nil && !os.IsNotExist(err) {
		t.Fatal(err)
	}
	defer func() {
		if hadModule {
			if err := ioutil.WriteFile("MODULE.bazel", origModuleData, 0666); err != nil {
				t.Errorf("error restoring MODULE.bazel: %v", err)
			}
			return
		}
		if err := os.Remove("MODULE.bazel"); err != nil && !os.IsNotExist(err) {
			t.Errorf("error removing MODULE.bazel: %v", err)
		}
	}()

	const moduleData = `
module(name = "go_download_sdk_bzlmod_test")

bazel_dep(name = "rules_go", repo_name = "io_bazel_rules_go")
# Keep @local_config_cc visible under bzlmod for Windows CI, where
# GO_BAZEL_TEST_BAZELFLAGS includes --extra_toolchains=@local_config_cc//...
bazel_dep(name = "rules_cc", version = "0.1.5")
local_path_override(
    module_name = "rules_go",
    path = "../tested_repo",
)

cc_configure = use_extension("@rules_cc//cc:extensions.bzl", "cc_configure_extension")
use_repo(cc_configure, "local_config_cc")

go_sdk = use_extension("@io_bazel_rules_go//go:extensions.bzl", "go_sdk")
go_sdk.download(
    name = "go_sdk",
    version = "1.26.0",
    experimental_build_compiler_from_source = True,
)
use_repo(go_sdk, "go_sdk")
`
	if err := ioutil.WriteFile("MODULE.bazel", []byte(moduleData), 0666); err != nil {
		t.Fatal(err)
	}

	if err := bazel_testing.RunBazel(
		"cquery",
		"//:version_test",
		"--enable_bzlmod",
		"--noenable_workspace",
	); err != nil {
		t.Fatal(err)
	}
}
