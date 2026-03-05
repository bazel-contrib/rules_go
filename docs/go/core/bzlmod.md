# Go with Bzlmod

This document describes how to use rules_go and Gazelle with Bazel's new external dependency subsystem [Bzlmod](https://bazel.build/external/overview#bzlmod), which is meant to replace `WORKSPACE` files eventually.
Usages of rules_go and Gazelle in `BUILD` files are not affected by this; refer to the existing documentation on rules and configuration options for them.

## Setup

Add the following lines to your `MODULE.bazel` file:

```starlark
bazel_dep(name = "rules_go", version = "0.57.0")
bazel_dep(name = "gazelle", version = "0.45.0")
```

The latest versions are always listed on https://registry.bazel.build/.

If you have WORKSPACE dependencies that reference rules_go and/or Gazelle, you can still use the legacy repository names for the two repositories:

```starlark
bazel_dep(name = "rules_go", version = "0.57.0", repo_name = "io_bazel_rules_go")
bazel_dep(name = "gazelle", version = "0.45.0", repo_name = "bazel_gazelle")
```

## Go SDKs

rules_go automatically downloads and registers a recent Go SDK, so unless a particular version is required, no manual steps are required.

To register a particular version of the Go SDK, use the `go_sdk` module extension:

```starlark
go_sdk = use_extension("@rules_go//go:extensions.bzl", "go_sdk")

# Download an SDK for the host OS & architecture as well as common remote execution
# platforms, using the version given from the `go.mod` file.
go_sdk.from_file(go_mod = "//:go.mod")

# Alternatively, use the version from a `go.work` file.
go_sdk.from_file(go_work = "//:go.work")

# Download an SDK for the host OS & architecture as well as common remote execution
# platforms, with a specific version.
go_sdk.download(version = "1.23.1")

# Alternatively, download an SDK for a fixed OS/architecture.
go_sdk.download(
    version = "1.23.1",
    goarch = "amd64",
    goos = "linux",
)

# Another alternative is to register the Go SDK installed on the host (see the nota bene below).
go_sdk.host()
```

Nota bene: The use of `go_sdk.host()` [may break builds](https://github.com/enola-dev/enola/issues/713) whenever the host Go version is upgraded
(because many OS package managers, such as Debian/Ubuntu's `apt`, distribute Go into a directory which contains the version, such as `/usr/lib/go-1.22/`).
As package upgrades happen outside of Bazel's control, this will lead to non-reproducible builds. Due to this, use of `go_sdk.host()` is discouraged.

When using `go_sdk.from_file()`, exactly one of `go_mod` or `go_work` must be specified.
Version extraction follows the same precedence for both file types: the `toolchain` directive takes precedence
over the `go` directive. If neither directive is present, `go.mod` has an implicit `go 1.16` line while
`go.work` has an implicit `go 1.18` line as per [Go Toolchains](https://go.dev/doc/toolchain#config) documentation.

You can register multiple Go SDKs and select which one to use on a per-target basis using [`go_cross_binary`](rules.md#go_cross_binary).
As long as you specify the `version` of an SDK, it will be downloaded lazily, that is, only when it is actually needed during a particular build.
The usual rules of [toolchain resolution](https://bazel.build/extending/toolchains#toolchain-resolution) apply, with SDKs registered in the root module taking precedence over those registered in dependencies.

### Using a Go SDK

By default, Go SDK repositories are created with mangled names and are not expected to be referenced directly.

For build actions, toolchain resolution is used to select the appropriate SDK for a given target.
[`go_cross_binary`](rules.md#go_cross_binary) can be used to influence the outcome of the resolution.

The `go` tool of the SDK registered for the host is available via the `@rules_go//go` target.
Prefer running it via this target over running `go` directly to ensure that all developers use the same version.
The `@rules_go//go` target can be used in scripts executed via `bazel run`, but cannot be used in build actions.
Note that `go` command arguments starting with `-` require the use of the double dash separator with `bazel run`:

```sh
bazel run @rules_go//go -- mod tidy -v
```

If you really do need direct access to a Go SDK, you can provide the `name` attribute on the `go_sdk.download` or `go_sdk.host` tag and then bring the repository with that name into scope via `use_repo`.
Note that modules using this attribute cannot be added to registries such as the Bazel Central Registry (BCR).
If you have a use case that would require this, please explain it in an issue.

### Configuring `nogo`

The `nogo` tool is a static analyzer for Go code that is run as part of compilation.
It is configured via an instance of the [`nogo`](/go/nogo.rst) rule, which can then be registered with the `go_sdk` extension:

```starlark
go_sdk = use_extension("@rules_go//go:extensions.bzl", "go_sdk")
go_sdk.nogo(nogo = "//:my_nogo")
```

By default, the `nogo` tool is executed for all Go targets in the main repository, but not any external repositories.
Each module can only provide at most one `go_sdk.nogo` tag and only the tag of the root module is honored.

It is also possible to include only or exclude particular packages from `nogo` analysis, using syntax that matches the `visibility` attribute on rules:

```starlark
go_sdk = use_extension("@rules_go//go:extensions.bzl", "go_sdk")
go_sdk.nogo(
    nogo = "//:my_nogo",
    includes = [
        "//:__subpackages__",
        "@my_own_go_dep//logic:__pkg__",
    ],
    excludes = [
        "//third_party:__subpackages__",
    ],
)
```

### Not yet supported

-   `go_local_sdk`

## Generating BUILD files

Add the following to your top-level BUILD file:

```starlark
load("@gazelle//:def.bzl", "gazelle")

gazelle(name = "gazelle")
```

If there is no `go.mod` file in the same directory as your top-level BUILD file, also add the following [Gazelle directive](https://github.com/bazelbuild/bazel-gazelle#directives) to that BUILD file to supply Gazelle with your Go module's path:

```starlark
# gazelle:prefix github.com/example/project
```

Then, use `bazel run //:gazelle` to (re-)generate BUILD files.

## External dependencies

External Go dependencies are managed by the `go_deps` module extension provided by Gazelle.
`go_deps` performs [Minimal Version Selection](https://go.dev/ref/mod#minimal-version-selection) on all transitive Go dependencies of all Bazel modules, so compared to the old WORKSPACE setup, every Bazel module only needs to declare its own Go dependencies.
For every major version of a Go module, there will only ever be a single version in the entire build, just as in regular Go module builds.

### Specifying external dependencies

Even though this is not a strict requirement, for interoperability with Go tooling that isn't Bazel-aware, it is recommended to manage Go dependencies via `go.mod`.
The `go_deps` extension parses this file directly, so external tooling such as `gazelle update-repos` is no longer needed.

Register the `go.mod` file with the `go_deps` extension as follows:

```starlark
go_deps = use_extension("@gazelle//:extensions.bzl", "go_deps")
go_deps.from_file(go_mod = "//:go.mod")

# All *direct* Go dependencies of the module have to be listed explicitly.
use_repo(
    go_deps,
    "com_github_gogo_protobuf",
    "com_github_golang_mock",
    "com_github_golang_protobuf",
    "org_golang_x_net",
)
```

When using Bazel 7.1.1 or higher, the [`@rules_go//go` target](#using-a-go-sdk) automatically updates the `use_repo` call whenever the `go.mod` file changes, using `bazel mod tidy`.
With older versions of Bazel, a warning with a fixup command will be emitted during a build if the `use_repo` call is out of date or missing.

Alternatively, you can specify a module extension tag to add an individual dependency:

```starlark
go_deps.module(
    path = "google.golang.org/grpc",
    sum = "h1:fPVVDxY9w++VjTZsYvXWqEf9Rqar/e+9zYfxKK+W+YU=",
    version = "v1.50.0",
)
```

#### Specifying Workspaces

The `go.work` functionality is supported by the `go_deps` module extension in Gazelle.

```starlark
go_deps = use_extension("@gazelle//:extensions.bzl", "go_deps")
go_deps.from_file(go_work = "//:go.work")

# All *direct* Go dependencies of all `go.mod` files referenced by the `go.work` file have to be listed explicitly.
use_repo(
    go_deps,
    "com_github_gogo_protobuf",
    "com_github_golang_mock",
    "com_github_golang_protobuf",
    "org_golang_x_net",
)
```

Limitations:

-   `go.work` is supported exclusively in the root module.
-   Dependencies that are indirect and depend on a go module specified in `go.work` will have that dependency diverge from the one in `go.work`. More details can be found here: https://github.com/bazelbuild/bazel-gazelle/issues/1797.

#### Depending on tools (Go 1.24+)

Go 1.24 introduced the [`tool` directive](https://tip.golang.org/doc/go1.24#tools), allowing you to specify tool dependencies directly in your `go.mod` like so:
```sh
bazel run @rules_go//go -- get -tool golang.org/x/tools/cmd/stringer
```

This will add a `tool` section in your `go.mod`:
```
tool golang.org/x/tools/cmd/stringer
```
as well as adding that tool as a dependency.

If you are using Gazelle >=0.47.0, then the tools you have added are exported as a dictionary named `GO_TOOLS` from `@gazelle//:go_tools.bzl`. This dictionary is in a suitable format for use by [`bazel_env.bzl`](https://github.com/buildbuddy-io/bazel_env.bzl), so you should be able to do the following to get all your repository’s tools into a `bazel_env` target:
```starlark
load("@bazel_env.bzl", "bazel_env")
load("@gazelle//:go_tools.bzl", "GO_TOOLS")
bazel_env(
    name = "env",
    tools = {
        // […]
    } | GO_TOOLS,
)
```

#### Depending on tools (pre Go 1.24)

If you need to depend on Go modules that are only used as tools, you can use the [`tools.go` technique](https://github.com/golang/go/issues/25922#issuecomment-1038394599):

1. In a new subdirectory of your repository, create a `tools.go` file that imports the tools' main packages:

    ```go
    //go:build tools
    // +build tools

    package my_tools

    import (
        _ "github.com/the/tool"
        _ "golang.org/x/tools/cmd/stringer"
    )
    ```

2. Run `bazel run @rules_go//go mod tidy` to populate the `go.mod` file with the dependencies of the tools.

Instead, if you want the tools' dependencies to be resolved independently of the dependencies of your regular code ([experimental](https://github.com/bazelbuild/bazel/issues/20186)):

2. Run `bazel run @rules_go//go mod init` in the directory containing the `tools.go` file to create a new `go.mod` file and then run `bazel run @rules_go//go mod tidy` in that directory.
3. Add `common --experimental_isolated_extension_usages` to your `.bazelrc` file to enable isolated usages of extensions.
4. Add an isolated usage of the `go_deps` extension to your module file:

    ```starlark
    go_tool_deps = use_extension("@gazelle//:extensions.bzl", "go_deps", isolate = True)
    go_tool_deps.from_file(go_mod = "//tools:go.mod")
    ```

### Managing `go.mod`

An initial `go.mod` file can be created via

```sh
bazel run @rules_go//go mod init github.com/example/project
```

A dependency can be added via

```sh
bazel run @rules_go//go get golang.org/x/text@v0.3.2
```

### Environment variables

Environment variables (such as `GOPROXY` and `GOPRIVATE`) required for fetching Go dependencies can be set as follows:

```starlark
go_deps.config(
   go_env = {
      "GOPRIVATE": "...",
   },
)
```

Variables set in this way are used by `go_deps` as well as `@rules_go//go`, with other variables inheriting their value from the host environment.
`go_env` does **not** affect Go build actions.

### Overrides

The root module can override certain aspects of the dependency resolution performed by the `go_deps` extension.

#### `replace`

[`replace` directives](https://go.dev/ref/mod#go-mod-file-replace) in `go.mod` can be used to replace particular or all versions of dependencies with other versions or entirely different modules.

```
replace(
    golang.org/x/net v1.2.3 => example.com/fork/net v1.4.5
    golang.org/x/mod => example.com/my/mod v1.4.5
    example.org/hello => ../../../fixtures/hello
)
```

#### Gazelle directives

Some external Go modules may require tweaking how Gazelle generates BUILD files for them via [Gazelle directives](https://github.com/bazelbuild/bazel-gazelle#directives).
The `go_deps` extension provides a dedicated `go_deps.gazelle_override` tag for this purpose:

```starlark
go_deps.gazelle_override(
    directives = [
        "gazelle:go_naming_convention go_default_library",
    ],
    path = "github.com/stretchr/testify",
)
```

If you need to use a `gazelle_override` to get a public Go module to build with Bazel, consider contributing the directives to the [public registry for default Gazelle overrides](https://github.com/bazelbuild/bazel-gazelle/blob/master/internal/bzlmod/default_gazelle_overrides.bzl) via a PR.
This will allow you to drop the `gazelle_override` tag and also makes the Go module usable in non-root Bazel modules.

Users can apply custom default directives or extra args to **all** modules, these can be added via a `go_deps.gazelle_default_attributes`. These will
disable/overwrite the [public registry overrides](https://github.com/bazelbuild/bazel-gazelle/blob/master/internal/bzlmod/default_gazelle_overrides.bzl).

```starlark
go_deps.gazelle_default_attributes(
    build_extra_args = [
        "-go_naming_convention_external=go_default_library",
    ],
    build_file_generation = "on",
    directives = [
        "gazelle:proto disable",
    ],
)
```

Overrides are applied with precedence decreasing in this order::

1. Specific `go_deps.gazelle_override` overrides per module
2. `go_deps.gazelle_default_attributes`, which will overwrite #3 (which now must be applied manually by users).
3. [public registry for default Gazelle overrides](https://github.com/bazelbuild/bazel-gazelle/blob/master/internal/bzlmod/default_gazelle_overrides.bzl)

It is recommended to avoid `go_deps.gazelle_default_attributes` and upstream the overrides to the [public registry for default Gazelle overrides](https://github.com/bazelbuild/bazel-gazelle/blob/master/internal/bzlmod/default_gazelle_overrides.bzl).

#### `go_deps.module_override`

A `go_deps.module_override` can be used to apply patches to a Go module:

```starlark
go_deps.module_override(
    patch_strip = 1,
    patches = [
        "//patches:testify.patch",
    ],
    path = "github.com/stretchr/testify",
)
```

#### `go_deps.archive_override`

A `go_deps.archive_override` can be used to replace a Go module with an archive fetched from a URL and is very similar to the `archive_override` for Bazel modules:

```starlark
go_deps.archive_override(
    urls = [
        "https://github.com/bazelbuild/buildtools/archive/ae8e3206e815d086269eb208b01f300639a4b194.tar.gz",
    ],
    patch_strip = 1,
    patches = [
        "//patches:buildtools.patch",
    ],
    strip_prefix = "buildtools-ae8e3206e815d086269eb208b01f300639a4b194",
    path = "github.com/bazelbuild/buildtools",
    sha256 = "05d7c3d2bd3cc0b02d15672fefa0d6be48c7aebe459c1c99dced7ac5e598508f",
)
```

### Not yet supported

-   Fetching dependencies from Git repositories
-   `go.mod` `exclude` directices
