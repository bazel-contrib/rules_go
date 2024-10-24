"""
  ["Make variable"]: https://docs.bazel.build/versions/master/be/make-variables.html
  [Bourne shell tokenization]: https://docs.bazel.build/versions/master/be/common-definitions.html#sh-tokenization
  [Gazelle]: https://github.com/bazelbuild/bazel-gazelle
  [GoArchive]: /go/providers.rst#GoArchive
  [GoPath]: /go/providers.rst#GoPath
  [GoInfo]: /go/providers.rst#GoInfo
  [build constraints]: https://golang.org/pkg/go/build/#hdr-Build_Constraints
  [cc_library deps]: https://docs.bazel.build/versions/master/be/c-cpp.html#cc_library.deps
  [cgo]: http://golang.org/cmd/cgo/
  [config_setting]: https://docs.bazel.build/versions/master/be/general.html#config_setting
  [data dependencies]: https://bazel.build/concepts/dependencies#data-dependencies
  [goarch]: /go/modes.rst#goarch
  [goos]: /go/modes.rst#goos
  [mode attributes]: /go/modes.rst#mode-attributes
  [nogo]: /go/nogo.rst#nogo
  [pure]: /go/modes.rst#pure
  [race]: /go/modes.rst#race
  [msan]: /go/modes.rst#msan
  [select]: https://docs.bazel.build/versions/master/be/functions.html#select
  [shard_count]: https://docs.bazel.build/versions/master/be/common-definitions.html#test.shard_count
  [static]: /go/modes.rst#static
  [test_arg]: https://docs.bazel.build/versions/master/user-manual.html#flag--test_arg
  [test_filter]: https://docs.bazel.build/versions/master/user-manual.html#flag--test_filter
  [test_env]: https://docs.bazel.build/versions/master/user-manual.html#flag--test_env
  [test_runner_fail_fast]: https://docs.bazel.build/versions/master/command-line-reference.html#flag--test_runner_fail_fast
  [define and register a C/C++ toolchain and platforms]: https://bazel.build/extending/toolchains#toolchain-definitions
  [bazel]: https://pkg.go.dev/github.com/bazelbuild/rules_go/go/tools/bazel?tab=doc
  [go_library]: #go_library
  [go_binary]: #go_binary
  [go_test]: #go_test
  [go_path]: #go_path
  [go_source]: #go_source
  [go_test]: #go_test
  [go_reset_target]: #go_reset_target
  [Examples]: examples.md#examples
  [Defines and stamping]: defines_and_stamping.md#defines-and-stamping
  [Stamping with the workspace status script]: defines_and_stamping.md#stamping-with-the-workspace-status-script
  [Embedding]: embedding.md#embedding
  [Cross compilation]: cross_compilation.md#cross-compilation
  [Platform-specific dependencies]: platform-specific_dependencies.md#platform-specific-dependencies

# Core Go rules

These are the core go rules, required for basic operation. The intent is that these rules are
sufficient to match the capabilities of the normal go tools.

## Additional resources
- ["Make variable"]
- [Bourne shell tokenization]
- [Gazelle]
- [GoArchive]
- [GoPath]
- [GoInfo]
- [build constraints]:
- [cc_library deps]
- [cgo]
- [config_setting]
- [data dependencies]
- [goarch]
- [goos]
- [mode attributes]
- [nogo]
- [pure]
- [race]
- [msan]
- [select]:
- [shard_count]
- [static]
- [test_arg]
- [test_filter]
- [test_env]
- [test_runner_fail_fast]
- [define and register a C/C++ toolchain and platforms]
- [bazel]


------------------------------------------------------------------------

Introduction
------------

Three core rules may be used to build most projects: [go_library], [go_binary],
and [go_test]. These rules reimplement the low level plumping commands of a normal
'go build' invocation: compiling package's source files to archives, then linking
archives into go binary.

[go_library] builds a single package. It has a list of source files
(specified with `srcs`) and may depend on other packages (with `deps`).
Each [go_library] has an `importpath`, which is the name used to import it
in Go source files.

[go_binary] also builds a single `main` package and links it into an
executable. It may embed the content of a [go_library] using the `embed`
attribute. Embedded sources are compiled together in the same package.
Binaries can be built for alternative platforms and configurations by setting
`goos`, `goarch`, and other attributes.

[go_test] builds a test executable. Like tests produced by `go test`, this
consists of three packages: an internal test package compiled together with
the library being tested (specified with `embed`), an external test package
compiled separately, and a generated test main package.

Here is an example of a Bazel build graph for a project using these core rules:

![](./buildgraph.svg)

By instrumenting the lower level go tooling, we can cache smaller, finer
artifacts with Bazel and thus, speed up incremental builds.

Rules
-----

"""

load("//go/private/rules:binary.bzl", _go_binary = "go_binary")
load("//go/private/rules:cross.bzl", _go_cross_binary = "go_cross_binary")
load("//go/private/rules:library.bzl", _go_library = "go_library")
load("//go/private/rules:source.bzl", _go_source = "go_source")
load("//go/private/rules:test.bzl", _go_test = "go_test")
load("//go/private/rules:transition.bzl", _go_reset_target = "go_reset_target")
load("//go/private/tools:path.bzl", _go_path = "go_path")

go_library = _go_library
go_binary = _go_binary
go_test = _go_test
go_source = _go_source
go_path = _go_path
go_cross_binary = _go_cross_binary
go_reset_target = _go_reset_target
