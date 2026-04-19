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
    version = "1.16",
)

`,
			optToWantVersion: map[string]string{"": "go1.16"},
		},
		{
			desc: "custom_archives",
			rule: `
load("@io_bazel_rules_go//go:deps.bzl", "go_download_sdk")

go_download_sdk(
    name = "go_sdk",
    sdks = {
        "darwin_amd64": ("go1.16.darwin-amd64.tar.gz", "6000a9522975d116bf76044967d7e69e04e982e9625330d9a539a8b45395f9a8"),
        "darwin_arm64": ("go1.16.darwin-arm64.tar.gz", "4dac57c00168d30bbd02d95131d5de9ca88e04f2c5a29a404576f30ae9b54810"),
        "linux_amd64": ("go1.16.linux-amd64.tar.gz", "013a489ebb3e24ef3d915abe5b94c3286c070dfe0818d5bca8108f1d6e8440d2"),
        "windows_amd64": ("go1.16.windows-amd64.zip", "5cc88fa506b3d5c453c54c3ea218fc8dd05d7362ae1de15bb67986b72089ce93"),
    },
)
`,
			optToWantVersion: map[string]string{"": "go1.16"},
		},
		{
			desc: "multiple_sdks",
			rule: `
load("@io_bazel_rules_go//go:deps.bzl", "go_download_sdk", "go_host_sdk")

go_download_sdk(
    name = "go_sdk",
    version = "1.16",
)
go_download_sdk(
    name = "go_sdk_1_17",
    version = "1.17",
)
go_download_sdk(
    name = "go_sdk_1_17_1",
    version = "1.17.1",
)
`,
			optToWantVersion: map[string]string{
				"": "go1.16",
				"--@io_bazel_rules_go//go/toolchain:sdk_version=remote": "go1.16",
				"--@io_bazel_rules_go//go/toolchain:sdk_version=1":      "go1.16",
				"--@io_bazel_rules_go//go/toolchain:sdk_version=1.17":   "go1.17",
				"--@io_bazel_rules_go//go/toolchain:sdk_version=1.17.0": "go1.17",
				"--@io_bazel_rules_go//go/toolchain:sdk_version=1.17.1": "go1.17.1",
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
    name = "go_sdk_1_17",
    version = "1.17",
)
go_download_sdk(
    name = "go_sdk_1_17_1",
    version = "1.17.1",
)
go_download_sdk(
    name = "go_sdk_with_experiments",
    version = "1.23.5",
	experiments = ["rangefunc"],
)
`,
			optToWantVersion: map[string]string{
				"": "go1.23.5",
				"--@io_bazel_rules_go//go/toolchain:sdk_name=go_sdk_1_17_1":           "go1.17.1",
				"--@io_bazel_rules_go//go/toolchain:sdk_name=go_sdk_1_17":             "go1.17",
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

func TestBootstrapPropagatesToolchainSource(t *testing.T) {
	for _, test := range []struct {
		name, bootstrapAttr string
	}{
		{
			name:          "experimental_bootstrap",
			bootstrapAttr: "experimental_bootstrap = True,",
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

			if !bytes.Contains(toolchainsBuildData, []byte(`sdk_source = "bootstrapped"`)) {
				t.Fatalf("go_sdk_toolchains should set sdk_source to bootstrapped when go_download_sdk(%s):\n%s", test.bootstrapAttr, toolchainsBuildData)
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
    experimental_bootstrap = True,
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
		"--@io_bazel_rules_go//go/toolchain:experimental_source=bootstrapped",
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
    experimental_bootstrap = True,
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
		"--@io_bazel_rules_go//go/toolchain:experimental_source=bootstrapped",
	); err != nil {
		t.Fatal(err)
	}
}
