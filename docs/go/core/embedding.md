## Embedding

The sources, dependencies, and data of a `go_library` may be *embedded*
within another `go_library`, `go_binary`, or `go_test` using the `embed`
attribute. The embedding package will be compiled into a single archive
file. The embedded package may still be compiled as a separate target.

A minimal example of embedding is below. In this example, the command `bazel
build :foo_and_bar` will compile `foo.go` and `bar.go` into a single
archive. `bazel build :bar` will compile only `bar.go`. Both libraries must
have the same `importpath`.

``` bzl
go_library(
    name = "foo_and_bar",
    srcs = ["foo.go"],
    embed = [":bar"],
    importpath = "example.com/foo",
)

go_library(
    name = "bar",
    srcs = ["bar.go"],
    importpath = "example.com/foo",
)
```

Embedding is most frequently used for tests and binaries. Go supports two
different kinds of tests. *Internal tests* (e.g., `package foo`) are compiled
into the same archive as the library under test and can reference unexported
definitions in that library. *External tests* (e.g., `package foo_test`) are
compiled into separate archives and may depend on exported definitions from the
internal test archive.

In order to compile the internal test archive, we *embed* the `go_library`
under test into a `go_test` that contains the test sources. The `go_test`
rule can automatically distinguish internal and external test sources, so they
can be listed together in `srcs`. The `go_library` under test does not
contain test sources. Other `go_binary` and `go_library` targets can depend
on it or embed it.

``` bzl
go_library(
    name = "foo_lib",
    srcs = ["foo.go"],
    importpath = "example.com/foo",
)

go_binary(
    name = "foo",
    embed = [":foo_lib"],
)

go_test(
    name = "go_default_test",
    srcs = [
        "foo_external_test.go",
        "foo_internal_test.go",
    ],
    embed = [":foo_lib"],
)
```

Embedding may also be used to add extra sources to a
`go_proto_library`.

``` bzl
proto_library(
    name = "foo_proto",
    srcs = ["foo.proto"],
)

go_proto_library(
    name = "foo_go_proto",
    importpath = "example.com/foo",
    proto = ":foo_proto",
)

go_library(
    name = "foo",
    srcs = ["extra.go"],
    embed = [":foo_go_proto"],
    importpath = "example.com/foo",
)
```

