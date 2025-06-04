Release notes

## `MODULE.bazel` code

```
bazel_dep(name = "rules_go", version = "1.2.3")

go_sdk = use_extension("@rules_go//go:extensions.bzl", "go_sdk")
go_sdk.download(version = "1.23")
```

## `WORKSPACE` code

```
load("@bazel_tools//tools/build_defs/repo:http.bzl", "http_archive")

http_archive(
    name = "io_bazel_rules_go",
    sha256 = "abcd1234",
    urls = [
        "https://mirror.bazel.build/github.com/bazel-contrib/rules_go/releases/download/v1.2.3/rules_go-v1.2.3.zip",
        "https://github.com/bazel-contrib/rules_go/releases/download/v1.2.3/rules_go-v1.2.3.zip",
    ],
)

load("@io_bazel_rules_go//go:deps.bzl", "go_register_toolchains", "go_rules_dependencies")

go_rules_dependencies()

go_register_toolchains(version = "1.23")
```
