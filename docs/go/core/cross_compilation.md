
## Cross compilation

rules_go can cross-compile Go projects to any platform the Go toolchain
supports. The simplest way to do this is by setting the `--platforms` flag on
the command line.

``` bash
$ bazel build --platforms=@io_bazel_rules_go//go/toolchain:linux_amd64 //my/project
```

You can replace `linux_amd64` in the example above with any valid
GOOS / GOARCH pair. To list all platforms, run this command:

``` bash
$ bazel query 'kind(platform, @io_bazel_rules_go//go/toolchain:all)'
```

By default, cross-compilation will cause Go targets to be built in "pure mode",
which disables cgo; cgo files will not be compiled, and C/C++ dependencies will
not be compiled or linked.

Cross-compiling cgo code is possible, but not fully supported. You will need to
[define and register a C/C++ toolchain and platforms](https://bazel.build/extending/toolchains#toolchain-definitions). You'll need to ensure it
works by building `cc_binary` and `cc_library` targets with the `--platforms`
command line flag set. Then, to build a mixed Go / C / C++ project, add
`pure = "off"` to your `go_binary` target and run Bazel with `--platforms`.
