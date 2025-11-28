Go rules for Bazel_
=====================

.. Links to external sites and pages
.. _//tests/core/cross: https://github.com/bazelbuild/rules_go/blob/master/tests/core/cross/BUILD.bazel
.. _Avoiding conflicts: proto/core.rst#avoiding-conflicts
.. _Bazel labels: https://docs.bazel.build/versions/master/build-ref.html#labels
.. _Bazel: https://bazel.build/
.. _Bazel Tutorial\: Build a Go Project: https://bazel.build/start/go
.. _Build modes: go/modes.rst
.. _Bzlmod: https://bazel.build/external/overview#bzlmod
.. _Go with Bzlmod: docs/go/core/bzlmod.md
.. _Go with WORKSPACE: docs/go/core/workspace.md
.. _Core rules: docs/go/core/rules.md
.. _Coverage: https://bazel.build/configure/coverage
.. _Dependencies: go/dependencies.rst
.. _Deprecation schedule: https://github.com/bazelbuild/rules_go/wiki/Deprecation-schedule
.. _Editor setup instructions: docs/editors.md
.. _examples/basic_gazelle: examples/basic_gazelle
.. _examples/hello: examples/hello
.. _Gopher Slack: https://invite.slack.golangbridge.org/
.. _gopls integration: docs/editors.md
.. _Overriding dependencies: go/dependencies.rst#overriding-dependencies
.. _Proto rules: proto/core.rst
.. _Protocol buffers: proto/core.rst
.. _Toolchains: go/toolchains.rst
.. _Using rules_go on Windows: windows.rst
.. _bazel-go-discuss: https://groups.google.com/forum/#!forum/bazel-go-discuss
.. _configuration transition: https://docs.bazel.build/versions/master/skylark/lib/transition.html
.. _gazelle update-repos: https://github.com/bazelbuild/bazel-gazelle#update-repos
.. _gazelle: https://github.com/bazelbuild/bazel-gazelle
.. _github.com/bazelbuild/bazel-gazelle: https://github.com/bazelbuild/bazel-gazelle
.. _github.com/bazelbuild/rules_go/go/tools/bazel: https://pkg.go.dev/github.com/bazelbuild/rules_go/go/tools/bazel?tab=doc
.. _nogo build-time static analysis: go/nogo.rst
.. _nogo: go/nogo.rst
.. _rules_go and Gazelle roadmap: https://github.com/bazelbuild/rules_go/wiki/Roadmap
.. _#bazel on Go Slack: https://gophers.slack.com/archives/C1SCQE54N
.. _#go on Bazel Slack: https://bazelbuild.slack.com/archives/CDBP88Z0D

.. Go rules
.. _go_binary: docs/go/core/rules.md#go_binary
.. _go_context: go/toolchains.rst#go_context
.. _go_deps: https://github.com/bazel-contrib/bazel-gazelle/blob/master/extensions.md#go_deps
.. _go_download_sdk: go/toolchains.rst#go_download_sdk
.. _go_host_sdk: go/toolchains.rst#go_host_sdk
.. _go_library: docs/go/core/rules.md#go_library
.. _go_local_sdk: go/toolchains.rst#go_local_sdk
.. _go_path: docs/go/core/rules.md#go_path
.. _go_proto_compiler: proto/core.rst#go_proto_compiler
.. _go_proto_library: proto/core.rst#go_proto_library
.. _go_register_toolchains: go/toolchains.rst#go_register_toolchains
.. _go_repository: https://github.com/bazelbuild/bazel-gazelle/blob/master/reference.md#go_repository
.. _go_rules_dependencies: go/dependencies.rst#go_rules_dependencies
.. _go_source: docs/go/core/rules.md#go_source
.. _go_test: docs/go/core/rules.md#go_test
.. _go_cross_binary: docs/go/core/rules.md#go_cross_binary
.. _go_toolchain: go/toolchains.rst#go_toolchain
.. _go_wrap_sdk: go/toolchains.rst#go_wrap_sdk
.. _gomock: docs/go/extras/extras.md#gomock

.. External rules
.. _git_repository: https://docs.bazel.build/versions/master/repo/git.html
.. _http_archive: https://docs.bazel.build/versions/master/repo/http.html#http_archive
.. _proto_library: https://github.com/bazelbuild/rules_proto

.. Issues
.. _#265: https://github.com/bazelbuild/rules_go/issues/265
.. _#721: https://github.com/bazelbuild/rules_go/issues/721
.. _#889: https://github.com/bazelbuild/rules_go/issues/889
.. _#1199: https://github.com/bazelbuild/rules_go/issues/1199
.. _#2775: https://github.com/bazelbuild/rules_go/issues/2775


Mailing list: `bazel-go-discuss`_

Slack: `#go on Bazel Slack`_, `#bazel on Go Slack`_

Contents
--------

* `Overview`_
* `Setup`_
* `FAQ`_

Documentation
~~~~~~~~~~~~~

* `Core rules`_

  * `go_binary`_
  * `go_library`_
  * `go_test`_
  * `go_source`_
  * `go_path`_
  * `go_cross_binary`_

* `Proto rules`_

  * `go_proto_library`_
  * `go_proto_compiler`_

* `Dependencies`_

  * `go_rules_dependencies`_
  * `go_repository`_ (Gazelle)

* `Toolchains`_

  * `go_register_toolchains`_
  * `go_download_sdk`_
  * `go_host_sdk`_
  * `go_local_sdk`_
  * `go_wrap_sdk`_
  * `go_toolchain`_
  * `go_context`_

* `Extra rules <docs/go/extras/extras.md>`_

  * `gomock`_

* `nogo build-time static analysis`_
* `Build modes <go/modes.rst>`_

Quick links
~~~~~~~~~~~

* `Editor setup instructions`_
* `rules_go and Gazelle roadmap`_
* `Deprecation schedule`_
* `Using rules_go on Windows`_

Overview
--------

These rules support:

* Building libraries, binaries, and tests (`go_library`_, `go_binary`_,
  `go_test`_)
* Go modules via `go_deps`_.
* Vendoring
* cgo
* Cross-compilation
* Generating BUILD files via gazelle_
* Build-time static code analysis via nogo_
* `Protocol buffers`_
* Remote execution
* `Coverage`_
* `gopls integration`_ for editor support
* Debugging

They currently do not support or have limited support for:

* C/C++ integration other than cgo (SWIG)

The Go rules are tested and supported on the following host platforms:

* Linux, macOS, Windows
* amd64, arm64

Users have reported success on several other platforms, but the rules are
only tested on those listed above.

Note: Since version v0.51.0, rules_go requires Bazel ≥ 6.5.0 to work.

The ``master`` branch is only guaranteed to work with the latest version of Bazel.


Setup
-----

To build Go code with Bazel, you will need:

* A recent version of Bazel.
* A C/C++ toolchain (if using cgo). Bazel will attempt to configure the
  toolchain automatically.
* Bash, ``patch``, ``cat``, and a handful of other Unix tools in ``PATH``.

You normally won't need a Go toolchain installed. Bazel will download one.

See `Using rules_go on Windows`_ for Windows-specific setup instructions.
Several additional tools need to be installed and configured.

If you're new to Bazel, read `Bazel Tutorial: Build a Go Project`_, which
introduces Bazel concepts and shows you how to set up a small Go workspace to
be built with Bazel.

For a quicker "hello world" example, see `examples/hello`_.

For an example that generates build files and retrieves external dependencies
using Gazelle, see `examples/basic_gazelle`_.

For more detailed `Bzlmod`_ documentation, see `Go with Bzlmod`_.

For legacy ``WORKSPACE`` instructions, see `Go with WORKSPACE`_.

FAQ
---

**Go**

* `Can I still use the go command?`_
* `Does this work with Go modules?`_
* `What's up with the go_default_library name?`_
* `How do I cross-compile?`_
* `How do I access testdata?`_
* `How do I access go_binary executables from go_test?`_

**Protocol buffers**

* `How do I avoid conflicts with protocol buffers?`_
* `Can I use a vendored gRPC with go_proto_library?`_

**Dependencies and testing**

* `How do I use different versions of dependencies?`_
* `How do I test a beta version of the Go SDK?`_

Can I still use the go command?
~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~

Yes, but not directly.

rules_go invokes the Go compiler and linker directly, based on the targets
described with `go_binary`_ and other rules. Bazel and rules_go together
fill the same role as the ``go`` command, so it's not necessary to use the
``go`` command in a Bazel workspace.

That said, it's usually still a good idea to follow conventions required by
the ``go`` command (e.g., one package per directory, package paths match
directory paths). Tools that aren't compatible with Bazel will still work,
and your project can be depended on by non-Bazel projects.

If you need to use the ``go`` command to perform tasks that Bazel doesn't cover
(such as adding a new dependency to ``go.mod``), you can use the following Bazel
invocation to run the ``go`` binary of the Bazel-configured Go SDK:

.. code:: bash

    bazel run @io_bazel_rules_go//go -- <args>

Prefer this to running ``go`` directly since it ensures that the version of Go
is identical to the one used by rules_go.

Does this work with Go modules?
~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~

Yes, but not directly. Bazel ignores ``go.mod`` files, and all package
dependencies must be expressed through ``deps`` attributes in targets
described with `go_library`_ and other rules.

You can download a Go module at a specific version as an external repository
using `go_repository`_, a workspace rule provided by gazelle_. This will also
generate build files using gazelle_.

You can import `go_repository`_ rules from a ``go.mod`` file using
`gazelle update-repos`_.

What's up with the go_default_library name?
~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~

This was used to keep import paths consistent in libraries that can be built
with ``go build`` before the ``importpath`` attribute was available.

In order to compile and link correctly, rules_go must know the Go import path
(the string by which a package can be imported) for each library. This is now
set explicitly with the ``importpath`` attribute. Before that attribute existed,
the import path was inferred by concatenating a string from a special
``go_prefix`` rule and the library's package and label name. For example, if
``go_prefix`` was ``github.com/example/project``, for a library
``//foo/bar:bar``, rules_go would infer the import path as
``github.com/example/project/foo/bar/bar``. The stutter at the end is
incompatible with ``go build``, so if the label name was ``go_default_library``,
the import path would not include it. So for the library
``//foo/bar:go_default_library``, the import path would be
``github.com/example/project/foo/bar``.

Since ``go_prefix`` was removed and the ``importpath`` attribute became
mandatory (see `#721`_), the ``go_default_library`` name no longer serves any
purpose. We may decide to stop using it in the future (see `#265`_).

How do I cross-compile?
~~~~~~~~~~~~~~~~~~~~~~~

You can cross-compile by setting the ``--platforms`` flag on the command line.
For example:

.. code::

  $ bazel build --platforms=@io_bazel_rules_go//go/toolchain:linux_amd64 //cmd

By default, cgo is disabled when cross-compiling. To cross-compile with cgo,
add a ``_cgo`` suffix to the target platform. You must register a
cross-compiling C/C++ toolchain with Bazel for this to work.

.. code::

  $ bazel build --platforms=@io_bazel_rules_go//go/toolchain:linux_amd64_cgo //cmd

Platform-specific sources with build tags or filename suffixes are filtered
automatically at compile time. You can selectively include platform-specific
dependencies with ``select`` expressions (Gazelle does this automatically).

.. code:: bzl

  go_library(
      name = "foo",
      srcs = [
          "foo_linux.go",
          "foo_windows.go",
      ],
      deps = select({
          "@io_bazel_rules_go//go/platform:linux_amd64": [
              "//bar_linux",
          ],
          "@io_bazel_rules_go//go/platform:windows_amd64": [
              "//bar_windows",
          ],
          "//conditions:default": [],
      }),
  )

To build a specific `go_binary`_ target for a target platform or using a
specific golang SDK version, use the `go_cross_binary`_ rule. This is useful
for producing multiple binaries for different platforms in a single build.

To build a specific `go_test`_ target for a target platform, set the
``goos`` and ``goarch`` attributes on that rule.

You can equivalently depend on a `go_binary`_ or `go_test`_ rule through
a Bazel `configuration transition`_ on ``//command_line_option:platforms``
(there are problems with this approach prior to rules_go 0.23.0).

How do I access testdata?
~~~~~~~~~~~~~~~~~~~~~~~~~

Bazel executes tests in a sandbox, which means tests don't automatically have
access to files. You must include test files using the ``data`` attribute.
For example, if you want to include everything in the ``testdata`` directory:

.. code:: bzl

  go_test(
      name = "foo_test",
      srcs = ["foo_test.go"],
      data = glob(["testdata/**"]),
      importpath = "github.com/example/project/foo",
  )

By default, tests are run in the directory of the build file that defined them.
Note that this follows the Go testing convention, not the Bazel convention
followed by other languages, which run in the repository root. This means
that you can access test files using relative paths. You can change the test
directory using the ``rundir`` attribute. See go_test_.

Gazelle will automatically add a ``data`` attribute like the one above if you
have a ``testdata`` directory *unless* it contains buildable .go files or
build files, in which case, ``testdata`` is treated as a normal package.

Note that on Windows, data files are not directly available to tests, since test
data files rely on symbolic links, and by default, Windows doesn't let
unprivileged users create symbolic links. You can use the
`github.com/bazelbuild/rules_go/go/tools/bazel`_ library to access data files.

How do I access go_binary executables from go_test?
~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~

The location where ``go_binary`` writes its executable file is not stable across
rules_go versions and should not be depended upon. The parent directory includes
some configuration data in its name. This prevents Bazel's cache from being
poisoned when the same binary is built in different configurations. The binary
basename may also be platform-dependent: on Windows, we add an .exe extension.

To depend on an executable in a ``go_test`` rule, reference the executable
in the ``data`` attribute (to make it visible), then expand the location
in ``args``. The real location will be passed to the test on the command line.
For example:

.. code:: bzl

  go_binary(
      name = "cmd",
      srcs = ["cmd.go"],
  )

  go_test(
      name = "cmd_test",
      srcs = ["cmd_test.go"],
      args = ["$(location :cmd)"],
      data = [":cmd"],
  )

See `//tests/core/cross`_ for a full example of a test that
accesses a binary.

Alternatively, you can set the ``out`` attribute of `go_binary`_ to a specific
filename. Note that when ``out`` is set, the binary won't be cached when
changing configurations.

.. code:: bzl

  go_binary(
      name = "cmd",
      srcs = ["cmd.go"],
      out = "cmd",
  )

  go_test(
      name = "cmd_test",
      srcs = ["cmd_test.go"],
      data = [":cmd"],
  )

How do I avoid conflicts with protocol buffers?
~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~

See `Avoiding conflicts`_ in the proto documentation.

Can I use a vendored gRPC with go_proto_library?
~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~

This is not supported. When using `go_proto_library`_ with the
``@io_bazel_rules_go//proto:go_grpc`` compiler, an implicit dependency is added
on ``@org_golang_google_grpc//:go_default_library``. If you link another copy of
the same package from ``//vendor/google.golang.org/grpc:go_default_library``
or anywhere else, you may experience conflicts at compile or run-time.

If you're using Gazelle with proto rule generation enabled, imports of
``google.golang.org/grpc`` will be automatically resolved to
``@org_golang_google_grpc//:go_default_library`` to avoid conflicts. The
vendored gRPC should be ignored in this case.

If you specifically need to use a vendored gRPC package, it's best to avoid
using ``go_proto_library`` altogether. You can check in pre-generated .pb.go
files and build them with ``go_library`` rules. Gazelle will generate these
rules when proto rule generation is disabled (add ``# gazelle:proto
disable_global`` to your root build file).

How do I use different versions of dependencies?
~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~

See `Overriding dependencies`_ for instructions on overriding repositories
declared in `go_rules_dependencies`_.

How do I test a beta version of the Go SDK?
~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~

rules_go only supports official releases of the Go SDK. However, you can still
test beta and RC versions by passing a ``version`` like ``"1.16beta1"`` to
`go_register_toolchains`_. See also `go_download_sdk`_.

.. code:: bzl

  load("@io_bazel_rules_go//go:deps.bzl", "go_register_toolchains", "go_rules_dependencies")

  go_rules_dependencies()

  go_register_toolchains(version = "1.17beta1")
