# Copyright 2026 The Bazel Authors. All rights reserved.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#    http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

def _experimental_bootstrap_go_sdk_impl(ctx):
    windows_constraint = ctx.attr._windows_constraint[platform_common.ConstraintValueInfo]
    is_windows_exec = ctx.target_platform_has_constraint(windows_constraint)
    sh_toolchain = ctx.toolchains["@rules_shell//shell:toolchain_type"]
    if not sh_toolchain or not sh_toolchain.path:
        fail("Go bootstrap SDK requires @rules_shell//shell:toolchain_type with a configured shell path")

    root_file = ctx.actions.declare_file(ctx.label.name + "/ROOT")
    version = ctx.actions.declare_file(ctx.label.name + "/VERSION")
    go_env = ctx.actions.declare_file(ctx.label.name + "/go.env")
    go = ctx.actions.declare_file(ctx.label.name + "/bin/go" + (".exe" if ctx.attr.goos == "windows" else ""))
    gofmt = ctx.actions.declare_file(ctx.label.name + "/bin/gofmt" + (".exe" if ctx.attr.goos == "windows" else ""))

    srcs = ctx.actions.declare_directory(ctx.label.name + "/src")
    libs = ctx.actions.declare_directory(ctx.label.name + "/pkg/" + ctx.attr.goos + "_" + ctx.attr.goarch)
    headers = ctx.actions.declare_directory(ctx.label.name + "/pkg/include")
    tools = ctx.actions.declare_directory(ctx.label.name + "/pkg/tool/" + ctx.attr.goos + "_" + ctx.attr.goarch)
    lib_misc = ctx.actions.declare_directory(ctx.label.name + "/lib")
    bootstrap_script = ctx.actions.declare_file(ctx.label.name + "_bootstrap_sdk.sh")

    args = ctx.actions.args()
    args.add(bootstrap_script)
    args.add(ctx.file.make_bash)
    args.add(ctx.file.make_bat)
    args.add(ctx.file.bootstrap_go)
    args.add(root_file)
    args.add(version)
    args.add(go_env)
    args.add(go)
    args.add(gofmt)
    args.add_all([srcs], expand_directories = False)
    args.add_all([libs], expand_directories = False)
    args.add_all([headers], expand_directories = False)
    args.add_all([tools], expand_directories = False)
    args.add_all([lib_misc], expand_directories = False)
    args.add(",".join(ctx.attr.experiments))
    args.add(ctx.attr.goos + "_" + ctx.attr.goarch)
    args.add("1" if is_windows_exec else "0")

    ctx.actions.write(
        output = bootstrap_script,
        content = """set -euo pipefail

MAKE_BASH="$1"
MAKE_BAT="$2"
BOOTSTRAP_GO="$3"
ROOT_FILE="$4"
VERSION_FILE="$5"
GO_ENV_FILE="$6"
GO_BIN="$7"
GOFMT_BIN="$8"
SRCS_OUT="$9"
LIBS_OUT="${10}"
HEADERS_OUT="${11}"
TOOLS_OUT="${12}"
LIB_OUT="${13}"
EXPERIMENTS="${14}"
GOOS_GOARCH="${15}"
IS_WINDOWS="${16}"

SRC_ROOT="$(cd "$(dirname "$MAKE_BASH")/.." && pwd)"
BOOTSTRAP_ROOT="$(cd "$(dirname "$BOOTSTRAP_GO")/.." && pwd)"
WORKDIR="${PWD}/_bootstrap_sdk_workdir_${RANDOM}_${RANDOM}"
ACTION_PATH="${PATH:-}"
if [[ -z "$ACTION_PATH" ]]; then
  ACTION_PATH="/usr/bin:/bin:/usr/sbin:/sbin"
fi

copy_dir() {
  local src="$1"
  local dst="$2"
  rm -rf "$dst"
  mkdir -p "$dst"
  if [[ -d "$src" ]]; then
    cp -RL "$src"/. "$dst"
  fi
}

rm -rf "$WORKDIR"
mkdir -p "$WORKDIR/goroot"
cp -RL "$SRC_ROOT"/. "$WORKDIR/goroot"
mkdir -p "$WORKDIR/home" "$WORKDIR/gocache"

(
  cd "$WORKDIR/goroot/src"
  if [[ "$IS_WINDOWS" == "1" ]]; then
    HOME="$WORKDIR/home" \
    GOCACHE="$WORKDIR/gocache" \
    PATH="$ACTION_PATH" \
    CGO_ENABLED=0 \
    GO111MODULE=off \
    GOENV=off \
    GOTELEMETRY=off \
    GOTOOLCHAIN=local \
    GOROOT_BOOTSTRAP="$BOOTSTRAP_ROOT" \
    GOEXPERIMENT="$EXPERIMENTS" \
    cmd.exe /c "$(basename "$MAKE_BAT")"
  else
    HOME="$WORKDIR/home" \
    GOCACHE="$WORKDIR/gocache" \
    PATH="$ACTION_PATH" \
    CGO_ENABLED=0 \
    GO111MODULE=off \
    GOENV=off \
    GOTELEMETRY=off \
    GOTOOLCHAIN=local \
    GOROOT_BOOTSTRAP="$BOOTSTRAP_ROOT" \
    GOEXPERIMENT="$EXPERIMENTS" \
    ./make.bash
  fi
)

mkdir -p "$(dirname "$ROOT_FILE")" "$(dirname "$GO_BIN")" "$(dirname "$GOFMT_BIN")"
: > "$ROOT_FILE"
cp "$WORKDIR/goroot/VERSION" "$VERSION_FILE"
if [[ -f "$WORKDIR/goroot/go.env" ]]; then
  cp "$WORKDIR/goroot/go.env" "$GO_ENV_FILE"
else
  : > "$GO_ENV_FILE"
fi
cp "$WORKDIR/goroot/bin/$(basename "$GO_BIN")" "$GO_BIN"
cp "$WORKDIR/goroot/bin/$(basename "$GOFMT_BIN")" "$GOFMT_BIN"

copy_dir "$WORKDIR/goroot/src" "$SRCS_OUT"
copy_dir "$WORKDIR/goroot/pkg/$GOOS_GOARCH" "$LIBS_OUT"
copy_dir "$WORKDIR/goroot/pkg/include" "$HEADERS_OUT"
copy_dir "$WORKDIR/goroot/pkg/tool/$GOOS_GOARCH" "$TOOLS_OUT"
copy_dir "$WORKDIR/goroot/lib" "$LIB_OUT"

rm -rf "$WORKDIR"
""",
    )

    ctx.actions.run(
        executable = sh_toolchain.path,
        inputs = depset(
            ctx.files.srcs +
            [bootstrap_script, ctx.file.make_bash, ctx.file.make_bat, ctx.file.bootstrap_go],
        ),
        outputs = [
            root_file,
            version,
            go_env,
            go,
            gofmt,
            srcs,
            libs,
            headers,
            tools,
            lib_misc,
        ],
        arguments = [args],
        toolchain = "@rules_shell//shell:toolchain_type",
        mnemonic = "GoBootstrapSDK",
        progress_message = "Bootstrapping Go SDK from source",
    )

    return [
        DefaultInfo(
            files = depset([go]),
            executable = go,
        ),
        OutputGroupInfo(
            root_file = depset([root_file]),
            version = depset([version]),
            go = depset([go]),
            srcs = depset([srcs]),
            libs = depset([libs]),
            headers = depset([headers]),
            tools = depset([tools, gofmt, go_env]),
            files = depset([go, gofmt, version, go_env, root_file, srcs, libs, headers, tools, lib_misc]),
        ),
    ]

_experimental_bootstrap_go_sdk = rule(
    implementation = _experimental_bootstrap_go_sdk_impl,
    attrs = {
        "srcs": attr.label_list(
            allow_files = True,
            mandatory = True,
        ),
        "make_bash": attr.label(
            allow_single_file = True,
            mandatory = True,
        ),
        "make_bat": attr.label(
            allow_single_file = True,
            mandatory = True,
        ),
        "bootstrap_go": attr.label(
            allow_single_file = True,
            mandatory = True,
        ),
        "goos": attr.string(mandatory = True),
        "goarch": attr.string(mandatory = True),
        "experiments": attr.string_list(),
        "_windows_constraint": attr.label(
            default = "@platforms//os:windows",
        ),
    },
    executable = True,
    toolchains = [
        config_common.toolchain_type("@rules_shell//shell:toolchain_type"),
    ],
)

def experimental_bootstrap_go_sdk(name, goos, goarch, experiments):
    impl_name = name + "_impl"

    _experimental_bootstrap_go_sdk(
        name = impl_name,
        srcs = native.glob(
            ["**"],
            exclude = [
                "BUILD.bazel",
                "version.bzl",
            ],
        ),
        make_bash = "src/make.bash",
        make_bat = "src/make.bat",
        bootstrap_go = "bin/go" + (".exe" if goos == "windows" else ""),
        goos = goos,
        goarch = goarch,
        experiments = experiments,
    )

    native.alias(
        name = name + "_go",
        actual = ":" + impl_name,
    )

    native.filegroup(
        name = name + "_root_file",
        srcs = [":" + impl_name],
        output_group = "root_file",
    )

    native.filegroup(
        name = name + "_srcs",
        srcs = [":" + impl_name],
        output_group = "srcs",
    )

    native.filegroup(
        name = name + "_libs",
        srcs = [":" + impl_name],
        output_group = "libs",
    )

    native.filegroup(
        name = name + "_headers",
        srcs = [":" + impl_name],
        output_group = "headers",
    )

    native.filegroup(
        name = name + "_tools",
        srcs = [":" + impl_name],
        output_group = "tools",
    )

    native.filegroup(
        name = name + "_files",
        srcs = [":" + impl_name],
        output_group = "files",
    )
