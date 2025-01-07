# Go with WORKSPACE

Bazel 7.0 and earlier supported declaring dependencies in a `WORKSPACE` or `WORKSPACE.bazel` file.
Bazel 8.0 also allows opting into this legacy feature with `--enable_workspace`.

## Initial Project Setup

Create a file at the top of your repository named `WORKSPACE`, and add the
snippet below (or add to your existing `WORKSPACE`). This tells Bazel to
fetch rules_go and its dependencies. Bazel will download a recent supported
Go toolchain and register it for use.

```starlark
load("@bazel_tools//tools/build_defs/repo:http.bzl", "http_archive")

http_archive(
    name = "io_bazel_rules_go",
    integrity = "sha256-M6zErg9wUC20uJPJ/B3Xqb+ZjCPn/yxFF3QdQEmpdvg=",
    urls = [
        "https://mirror.bazel.build/github.com/bazelbuild/rules_go/releases/download/v0.48.0/rules_go-v0.48.0.zip",
        "https://github.com/bazelbuild/rules_go/releases/download/v0.48.0/rules_go-v0.48.0.zip",
    ],
)

load("@io_bazel_rules_go//go:deps.bzl", "go_register_toolchains", "go_rules_dependencies")

go_rules_dependencies()

go_register_toolchains(version = "1.23.1")
```

You can use rules_go at `master` by using `git_repository` instead of
`http_archive` and pointing to a recent commit.

Add a file named `BUILD.bazel` in the root directory of your project.
You'll need a build file in each directory with Go code, but you'll also need
one in the root directory, even if your project doesn't have Go code there.
For a "Hello, world" binary, the file should look like this:

```starlark
load("@io_bazel_rules_go//go:def.bzl", "go_binary")

go_binary(
    name = "hello",
    srcs = ["hello.go"],
)
```

You can build this target with `bazel build //:hello`.

## Generating build files

If your project can be built with `go build`, you can generate and update your
build files automatically using gazelle.

Add the `bazel_gazelle` repository and its dependencies to your
`WORKSPACE`. It should look like this:

```starlark
load("@bazel_tools//tools/build_defs/repo:http.bzl", "http_archive")

http_archive(
    name = "io_bazel_rules_go",
    integrity = "sha256-M6zErg9wUC20uJPJ/B3Xqb+ZjCPn/yxFF3QdQEmpdvg=",
    urls = [
        "https://mirror.bazel.build/github.com/bazelbuild/rules_go/releases/download/v0.48.0/rules_go-v0.48.0.zip",
        "https://github.com/bazelbuild/rules_go/releases/download/v0.48.0/rules_go-v0.48.0.zip",
    ],
)

http_archive(
    name = "bazel_gazelle",
    integrity = "sha256-12v3pg/YsFBEQJDfooN6Tq+YKeEWVhjuNdzspcvfWNU=",
    urls = [
        "https://mirror.bazel.build/github.com/bazelbuild/bazel-gazelle/releases/download/v0.37.0/bazel-gazelle-v0.37.0.tar.gz",
        "https://github.com/bazelbuild/bazel-gazelle/releases/download/v0.37.0/bazel-gazelle-v0.37.0.tar.gz",
    ],
)

load("@io_bazel_rules_go//go:deps.bzl", "go_register_toolchains", "go_rules_dependencies")
load("@bazel_gazelle//:deps.bzl", "gazelle_dependencies")

go_rules_dependencies()

go_register_toolchains(version = "1.23.1")

gazelle_dependencies()
```

Add the code below to the `BUILD.bazel` file in your project's root directory.
Replace the string after `prefix` with an import path prefix that matches your
project. It should be the same as your module path, if you have a `go.mod`
file.

```starlark
load("@bazel_gazelle//:def.bzl", "gazelle")

# gazelle:prefix github.com/example/project
gazelle(name = "gazelle")
```

This declares a `gazelle` binary rule, which you can run using the command
below:

```bash
bazel run //:gazelle
```

This will generate a `BUILD.bazel` file with `go_library`, `go_binary`, and
`go_test` targets for each package in your project. You can run the same
command in the future to update existing build files with new source files,
dependencies, and options.

## Writing build files by hand

If your project doesn't follow `go build` conventions or you prefer not to use
gazelle_, you can write build files by hand.

In each directory that contains Go code, create a file named `BUILD.bazel`
Add a `load` statement at the top of the file for the rules you use.

```starlark
load("@io_bazel_rules_go//go:def.bzl", "go_binary", "go_library", "go_test")
```

For each library, add a `go_library` rule like the one below.  Source files are
listed in the `srcs` attribute. Imported packages outside the standard library
are listed in the `deps` attribute using `Bazel labels` that refer to
corresponding `go_library` rules. The library's import path must be specified
with the `importpath` attribute.

```starlark
go_library(
    name = "foo_library",
    srcs = [
        "a.go",
        "b.go",
    ],
    importpath = "github.com/example/project/foo",
    deps = [
        "//tools",
        "@org_golang_x_utils//stuff",
    ],
    visibility = ["//visibility:public"],
)
```

For tests, add a `go_test` rule like the one below. The library being tested
should be listed in an `embed` attribute.

```starlark
go_test(
    name = "foo_test",
    srcs = [
        "a_test.go",
        "b_test.go",
    ],
    embed = [":foo_lib"],
    deps = [
        "//testtools",
        "@org_golang_x_utils//morestuff",
    ],
)
```

For binaries, add a `go_binary` rule like the one below.

```starlark
go_binary(
    name = "foo",
    srcs = ["main.go"],
)
```

## Adding external repositories

For each Go repository, add a `go_repository` rule to `WORKSPACE` like the
one below.  This rule comes from the Gazelle repository, so you will need to
load it. `gazelle update-repos` can generate or update these rules
automatically from a go.mod or Gopkg.lock file.

```starlark
load("@bazel_tools//tools/build_defs/repo:http.bzl", "http_archive")

# Download the Go rules.
http_archive(
    name = "io_bazel_rules_go",
    integrity = "sha256-M6zErg9wUC20uJPJ/B3Xqb+ZjCPn/yxFF3QdQEmpdvg=",
    urls = [
        "https://mirror.bazel.build/github.com/bazelbuild/rules_go/releases/download/v0.48.0/rules_go-v0.48.0.zip",
        "https://github.com/bazelbuild/rules_go/releases/download/v0.48.0/rules_go-v0.48.0.zip",
    ],
)

# Download Gazelle.
http_archive(
    name = "bazel_gazelle",
    integrity = "sha256-12v3pg/YsFBEQJDfooN6Tq+YKeEWVhjuNdzspcvfWNU=",
    urls = [
        "https://mirror.bazel.build/github.com/bazelbuild/bazel-gazelle/releases/download/v0.37.0/bazel-gazelle-v0.37.0.tar.gz",
        "https://github.com/bazelbuild/bazel-gazelle/releases/download/v0.37.0/bazel-gazelle-v0.37.0.tar.gz",
    ],
)

# Load macros and repository rules.
load("@io_bazel_rules_go//go:deps.bzl", "go_register_toolchains", "go_rules_dependencies")
load("@bazel_gazelle//:deps.bzl", "gazelle_dependencies", "go_repository")

# Declare Go direct dependencies.
go_repository(
    name = "org_golang_x_net",
    importpath = "golang.org/x/net",
    sum = "h1:7EYJ93RZ9vYSZAIb2x3lnuvqO5zneoD6IvWjuhfxjTs=",
    version = "v0.23.0",
)

# Declare indirect dependencies and register toolchains.
go_rules_dependencies()

go_register_toolchains(version = "1.23.1")

gazelle_dependencies()
```

# protobuf and gRPC

To generate code from protocol buffers, you'll need to add a dependency on
`com_google_protobuf` to your `WORKSPACE`.

```starlark
load("@bazel_tools//tools/build_defs/repo:http.bzl", "http_archive")

http_archive(
    name = "com_google_protobuf",
    sha256 = "535fbf566d372ccf3a097c374b26896fa044bf4232aef9cab37bd1cc1ba4e850",
    strip_prefix = "protobuf-3.15.0",
    urls = [
        "https://mirror.bazel.build/github.com/protocolbuffers/protobuf/archive/v3.15.0.tar.gz",
        "https://github.com/protocolbuffers/protobuf/archive/v3.15.0.tar.gz",
    ],
)

load("@com_google_protobuf//:protobuf_deps.bzl", "protobuf_deps")

protobuf_deps()
```

You'll need a C/C++ toolchain registered for the execution platform (the
platform where Bazel runs actions) to build protoc.

The `proto_library` rule is provided by the `rules_proto` repository.
`protoc-gen-go`, the Go proto compiler plugin, is provided by the
`com_github_golang_protobuf` repository. Both are declared by
`go_rules_dependencies`. You won't need to declare an explicit dependency
unless you specifically want to use a different version. See `Overriding
dependencies` for instructions on using a different version.

gRPC dependencies are not declared by default (there are too many). You can
declare them in WORKSPACE using `go_repository`. You may want to use
`gazelle update-repos` to import them from ``go.mod``.

See [Proto dependencies], [gRPC dependencies] for more information. See also
[Avoiding conflicts].

Once all dependencies have been registered, you can declare `proto_library`
and `go_proto_library` rules to generate and compile Go code from .proto
files.

```starlark
load("@rules_proto//proto:defs.bzl", "proto_library")
load("@io_bazel_rules_go//proto:def.bzl", "go_proto_library")

proto_library(
    name = "foo_proto",
    srcs = ["foo.proto"],
    deps = ["//bar:bar_proto"],
    visibility = ["//visibility:public"],
)

go_proto_library(
    name = "foo_go_proto",
    importpath = "github.com/example/protos/foo_proto",
    protos = [":foo_proto"],
    visibility = ["//visibility:public"],
)
```

A `go_proto_library` target may be imported and depended on like a normal
`go_library`.

Note that recent versions of rules_go support both APIv1
(`github.com/golang/protobuf`) and APIv2 (`google.golang.org/protobuf`).
By default, code is generated with
`github.com/golang/protobuf/cmd/protoc-gen-gen` for compatibility with both
interfaces. Client code may import use either runtime library or both.

[Proto dependencies]: /go/dependencies.rst#proto-dependencies
[gRPC dependencies]: /go/dependencies.rst#grpc-dependencies
[Avoiding conflicts]: /proto/core.rst#avoiding-conflicts
