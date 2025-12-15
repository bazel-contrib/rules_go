Go toolchains
=============

.. _Args: https://docs.bazel.build/versions/master/skylark/lib/Args.html
.. _Bazel toolchains: https://docs.bazel.build/versions/master/toolchains.html
.. _Go website: https://golang.org/
.. _GoArchive: providers.rst#goarchive
.. _GoSDK: providers.rst#gosdk
.. _GoInfo: providers.rst#gosource
.. _binary distribution: https://golang.org/dl/
.. _compilation modes: modes.rst#compilation-modes
.. _control the version: `Forcing the Go version`_
.. _core: core.rst
.. _forked version of Go: `Registering a custom SDK`_
.. _go assembly: https://golang.org/doc/asm
.. _go sdk rules: `The SDK`_
.. _go/platform/list.bzl: platform/list.bzl
.. _installed SDK: `Using the installed Go sdk`_
.. _nogo: nogo.rst#nogo
.. _register: Registration_
.. _register_toolchains: https://docs.bazel.build/versions/master/skylark/lib/globals.html#register_toolchains
.. _toolchain resolution: https://bazel.build/extending/toolchains#toolchain-resolution

.. role:: param(kbd)
.. role:: type(emphasis)
.. role:: value(code)
.. |mandatory| replace:: **mandatory value**

The Go toolchain is at the heart of the Go rules, and is the mechanism used to
customize the behavior of the core_ Go rules.

.. contents:: :depth: 2

-----

Overview
--------

The Go toolchain consists of three main layers: `the SDK`_, `the toolchain`_,
and `the context`_.

The SDK
~~~~~~~

The Go SDK (more commonly known as the Go distribution) is a directory tree
containing sources for the Go toolchain and standard library and pre-compiled
binaries for the same. You can download this from by visiting the `Go website`_
and downloading a `binary distribution`_.

There are several Bazel rules for obtaining and configuring a Go SDK:

* `go_download_sdk`_: downloads a toolchain for a specific version of Go for a
  specific operating system and architecture.
* `go_host_sdk`_: uses the toolchain installed on the system where Bazel is
  run. The toolchain's location is specified with the ``GOROOT`` or by running
  ``go env GOROOT``.
* `go_local_sdk`_: like `go_host_sdk`_, but uses the toolchain in a specific
  directory on the host system.
* `go_wrap_sdk`_: configures a toolchain downloaded with another Bazel
  repository rule.

By default, if none of the above rules are used, the `go_register_toolchains`_
function creates a repository named ``@go_sdk`` using `go_download_sdk`_, using
a recent version of Go for the host operating system and architecture.

SDKs are specific to a host platform (e.g., ``linux_amd64``) and a version of
Go. They may target all platforms that Go supports. The Go SDK is naturally
cross compiling.

By default, all ``go_binary``, ``go_test``, etc. rules will use the first declared
Go SDK. If you would like to build a target using a specific Go SDK version, first
ensure that you have declared a Go SDK of that version using one of the above rules
(`go_download_sdk`_, `go_host_sdk`_, `go_local_sdk`_, `go_wrap_sdk`_). Then you
can specify the sdk version to build with when running a ``bazel build`` by passing
the flag ``--@io_bazel_rules_go//go/toolchain:sdk_version="version"`` where
``"version"`` is the SDK version you would like to build with, eg. ``"1.18.3"``.
The SDK version can omit the patch, or include a prerelease part, eg. ``"1"``,
``"1.18"``, ``"1.18.0"``, and ``"1.19.0beta1"`` are all valid values for ``sdk_version``.
When ``go_host_sdk`` is used, ``"version"`` can be set to ``host`` to refer to the host Go SDK.
It can also be set ``remote`` to match any non-host version.

If you would like to use a specific Go SDK target, pass the flag ``--@io_bazel_rules_go//go/toolchain:sdk_name="name"``.
This can be useful if there are several `go_download_sdk`_ / `go_host_sdk`_ / `go_local_sdk`_ / `go_wrap_sdk`_
with the same Go SDK version, but have different experiments enabled or patches applied:

.. code:: bzl

    # WORKSPACE

    go_download_sdk(
      name = "go_sdk",
      version = "1.23.5",
    )

    # select with bazel build --@io_bazel_rules_go//go/toolchain:sdk_name=go_sdk_with_rangefunc
    go_download_sdk(
      name = "go_sdk_with_rangefunc",
      version = "1.23.5",
      experiments = ["rangefunc"],
    )

    # MODULE.bazel

    go_sdk.download(
      name = "go_sdk",
      version = "1.23.5",
    )

    # select with bazel build --@io_bazel_rules_go//go/toolchain:sdk_name=go_sdk_with_rangefunc
    go_sdk.download(
      name = "go_sdk_with_rangefunc",
      version = "1.23.5",
      experiments = ["rangefunc"],
    )

The toolchain
~~~~~~~~~~~~~

The workspace rules above declare `Bazel toolchains`_ with `go_toolchain`_
implementations for each target platform that Go supports. Wrappers around
the rules register these toolchains automatically. Bazel will select a
registered toolchain automatically based on the execution and target platforms,
specified with ``--host_platform`` and ``--platforms``, respectively.

The workspace rules define the toolchains in a separate repository from the
SDK. For example, if the SDK repository is `@go_sdk`, the toolchains will be
defined in `@go_sdk_toolchains`. The `@go_sdk_toolchains` repository must be
eagerly fetched in order to register the toolchain, but fetching the `@go_sdk`
repository may be delayed until the toolchain is needed to build something. To
activate lazily fetching the SDK, you must provide a `version` attribute to the
workspace rule that defines the SDK (`go_download_sdk`, `go_host_sdk`, `go_local_sdk`,
`go_wrap_sdk`, or `go_register_toolchains`). The value must match the actual
version of the SDK; rules_go will validate this when the toolchain is used.

The toolchain itself should be considered opaque. You should only access
its contents through `the context`_.

The context
~~~~~~~~~~~

The context is the type you need if you are writing custom rules that need
to be compatible with rules_go. It provides information about the SDK, the
toolchain, and the standard library. It also provides a convenient way to
declare mode-specific files, and to create actions for compiling, linking,
and more.

Customizing
-----------

Normal usage
~~~~~~~~~~~~

This is an example of normal usage for the other examples to be compared
against. This will download and use a specific version of Go for the host
platform.

.. code:: bzl

    # WORKSPACE

    load("@io_bazel_rules_go//go:deps.bzl", "go_rules_dependencies", "go_register_toolchains")

    go_rules_dependencies()

    go_register_toolchains(version = "1.23.1")


Using the installed Go SDK
~~~~~~~~~~~~~~~~~~~~~~~~~~

You can use the Go SDK that's installed on the system where Bazel is running.
This may result in faster builds, since there's no need to download an SDK,
but builds won't be reproducible across systems with different SDKs installed.

.. code:: bzl

    # WORKSPACE

    load("@io_bazel_rules_go//go:deps.bzl", "go_rules_dependencies", "go_register_toolchains")

    go_rules_dependencies()

    go_register_toolchains(version = "host")


Registering a custom SDK
~~~~~~~~~~~~~~~~~~~~~~~~

If you download the SDK through another repository rule, you can configure
it with ``go_wrap_sdk``. It must still be named ``go_sdk``, but this is a
temporary limitation that will be removed in the future.

.. code:: bzl

    # WORKSPACE

    load("@io_bazel_rules_go//go:deps.bzl", "go_rules_dependencies", "go_register_toolchains", "go_wrap_sdk")

    unknown_download_sdk(
        name = "go",
        ...,
    )

    go_wrap_sdk(
        name = "go_sdk",
        root_file = "@go//:README.md",
    )

    go_rules_dependencies()

    go_register_toolchains()


Writing new Go rules
~~~~~~~~~~~~~~~~~~~~

If you are writing a new Bazel rule that uses the Go toolchain, you need to
do several things to ensure you have full access to the toolchain and common
dependencies.

* Declare a dependency on a toolchain of type
  ``@io_bazel_rules_go//go:toolchain``. Bazel will select an appropriate,
  registered toolchain automatically.
* Declare an implicit attribute named ``_go_context_data`` that defaults to
  ``@io_bazel_rules_go//:go_context_data``. This target gathers configuration
  information and several common dependencies.
* Use the ``go_context`` function to gain access to `the context`_. This is
  your main interface to the Go toolchain.

.. code:: bzl

    load("@io_bazel_rules_go//go:def.bzl", "go_context")

    def _my_rule_impl(ctx):
        go = go_context(ctx)
        ...

    my_rule = rule(
        implementation = _my_rule_impl,
        attrs = {
            ...
            "_go_context_data": attr.label(
                default = "@io_bazel_rules_go//:go_context_data",
            ),
        },
        toolchains = ["@io_bazel_rules_go//go:toolchain"],
    )


Rules and functions
-------------------

go_register_toolchains
~~~~~~~~~~~~~~~~~~~~~~

Installs the Go toolchains. If :param:`version` is specified, it sets the
SDK version to use (for example, :value:`"1.15.5"`).

+--------------------------------+-----------------------------+-----------------------------------+
| **Name**                       | **Type**                    | **Default value**                 |
+--------------------------------+-----------------------------+-----------------------------------+
| :param:`version`               | :type:`string`              | |mandatory|                       |
+--------------------------------+-----------------------------+-----------------------------------+
| Specifies the version of Go to download if one has not been declared.                            |
|                                                                                                  |
| If a toolchain was already declared with `go_download_sdk`_ or a similar rule,                   |
| this parameter may not be set.                                                                   |
|                                                                                                  |
| Normally this is set to a Go version like :value:`"1.15.5"`. It may also be                      |
| set to :value:`"host"`, which will cause rules_go to use the Go toolchain                        |
| installed on the host system (found using ``GOROOT`` or ``PATH``).                               |
|                                                                                                  |
| If ``version`` is specified and is not set to :value:`"host"`, the SDK will be fetched only when |
| the build uses a Go toolchain and `toolchain resolution`_ results in  this SDK being chosen.     |
| Otherwise it will be fetched unconditionally.                                                    |
+--------------------------------+-----------------------------+-----------------------------------+
| :param:`nogo`                  | :type:`label`               | :value:`None`                     |
+--------------------------------+-----------------------------+-----------------------------------+
| The ``nogo`` attribute refers to a nogo_ rule that builds a binary                               |
| used for static analysis. The ``nogo`` binary will be used alongside the                         |
| Go compiler when building packages.                                                              |
+--------------------------------+-----------------------------+-----------------------------------+
| :param:`experiments`           | :type:`string_list`         | :value:`[]`                       |
+--------------------------------+-----------------------------+-----------------------------------+
| Go experiments to enable via `GOEXPERIMENT`.                                                     |
+--------------------------------+-----------------------------+-----------------------------------+

go_download_sdk
~~~~~~~~~~~~~~~

This downloads a Go SDK for use in toolchains.

+--------------------------------+-----------------------------+---------------------------------------------+
| **Name**                       | **Type**                    | **Default value**                           |
+--------------------------------+-----------------------------+---------------------------------------------+
| :param:`name`                  | :type:`string`              | |mandatory|                                 |
+--------------------------------+-----------------------------+---------------------------------------------+
| A unique name for this SDK. This should almost always be :value:`go_sdk` if                                |
| you want the SDK to be used by toolchains.                                                                 |
+--------------------------------+-----------------------------+---------------------------------------------+
| :param:`goos`                  | :type:`string`              | :value:`None`                               |
+--------------------------------+-----------------------------+---------------------------------------------+
| The operating system the binaries in the SDK are intended to run on.                                       |
| By default, this is detected automatically, but if you're building on                                      |
| an unusual platform, or if you're using remote execution and the execution                                 |
| platform is different than the host, you may need to specify this explictly.                               |
+--------------------------------+-----------------------------+---------------------------------------------+
| :param:`goarch`                | :type:`string`              | :value:`None`                               |
+--------------------------------+-----------------------------+---------------------------------------------+
| The architecture the binaries in the SDK are intended to run on.                                           |
| By default, this is detected automatically, but if you're building on                                      |
| an unusual platform, or if you're using remote execution and the execution                                 |
| platform is different than the host, you may need to specify this explictly.                               |
+--------------------------------+-----------------------------+---------------------------------------------+
| :param:`version`               | :type:`string`              | :value:`latest Go version`                  |
+--------------------------------+-----------------------------+---------------------------------------------+
| The version of Go to download, for example ``1.12.5``. If unspecified,                                     |
| ``go_download_sdk`` will list available versions of Go from golang.org, then                               |
| pick the highest version. If ``version`` is specified but ``sdks`` is                                      |
| unspecified, ``go_download_sdk`` will list available versions on golang.org                                |
| to determine the correct file name and SHA-256 sum.                                                        |
| If ``version`` is specified, the SDK will be fetched only when the build uses a Go toolchain and           |
| `toolchain resolution`_ results in this SDK being chosen. Otherwise it will be fetched unconditionally.    |
+--------------------------------+-----------------------------+---------------------------------------------+
| :param:`urls`                  | :type:`string_list`         | :value:`[https://dl.google.com/go/{}]`      |
+--------------------------------+-----------------------------+---------------------------------------------+
| A list of mirror urls to the binary distribution of a Go SDK. These must contain the `{}`                  |
| used to substitute the sdk filename being fetched (using `.format`.                                        |
| It defaults to the official repository :value:`"https://dl.google.com/go/{}"`.                             |
|                                                                                                            |
| This attribute is seldom used. It is only needed for downloading Go from                                   |
| an alternative location (for example, an internal mirror).                                                 |
+--------------------------------+-----------------------------+---------------------------------------------+
| :param:`strip_prefix`          | :type:`string`              | :value:`"go"`                               |
+--------------------------------+-----------------------------+---------------------------------------------+
| A directory prefix to strip from the extracted files.                                                      |
| Used with ``urls``.                                                                                        |
+--------------------------------+-----------------------------+---------------------------------------------+
| :param:`sdks`                  | :type:`string_list_dict`    | :value:`see description`                    |
+--------------------------------+-----------------------------+---------------------------------------------+
| This consists of a set of mappings from the host platform tuple to a list of filename and                  |
| sha256 for that file. The filename is combined the :param:`urls` to produce the final download             |
| urls to use.                                                                                               |
|                                                                                                            |
| This option is seldom used. It is only needed for downloading a modified                                   |
| Go distribution (with a different SHA-256 sum) or a version of Go                                          |
| not supported by rules_go (for example, a beta or release candidate).                                      |
+--------------------------------+-----------------------------+---------------------------------------------+
| :param:`patches`               | :type:`label_list`          | :value:`[]`                                 |
+--------------------------------+-----------------------------+---------------------------------------------+
| A list of files that are to be applied to go sdk. By default, it uses the Bazel-native patch               |
| implementation which doesn't support fuzz match and binary patch, but Bazel will fall back to use          |
| patch command line tool if `patch_tool` attribute is specified.                                            |
+--------------------------------+-----------------------------+---------------------------------------------+
| :param:`patch_strip`           | :type:`int`                 | :value:`0`                                  |
+--------------------------------+-----------------------------+---------------------------------------------+
| The number of leading slashes to be stripped from the file name in thepatches.                             |
+--------------------------------+-----------------------------+---------------------------------------------+

**Example**:

.. code:: bzl

    load(
        "@io_bazel_rules_go//go:deps.bzl",
        "go_download_sdk",
        "go_register_toolchains",
        "go_rules_dependencies",
    )

    go_download_sdk(
        name = "go_sdk",
        goos = "linux",
        goarch = "amd64",
        version = "1.18.1",
        sdks = {
            # NOTE: In most cases the whole sdks attribute is not needed.
            # There are 2 "common" reasons you might want it:
            #
            # 1. You need to use an modified GO SDK, or an unsupported version
            #    (for example, a beta or release candidate)
            #
            # 2. You want to avoid the dependency on the index file for the
            #    SHA-256 checksums. In this case, You can get the expected
            #    filenames and checksums from https://go.dev/dl/
            "linux_amd64": ("go1.18.1.linux-amd64.tar.gz", "b3b815f47ababac13810fc6021eb73d65478e0b2db4b09d348eefad9581a2334"),
            "darwin_amd64": ("go1.18.1.darwin-amd64.tar.gz", "3703e9a0db1000f18c0c7b524f3d378aac71219b4715a6a4c5683eb639f41a4d"),
        },
        patch_strip = 1,
        patches = [
            "//patches:cgo_issue_fix.patch",
        ]
    )

    go_rules_dependencies()

    go_register_toolchains()

go_host_sdk
~~~~~~~~~~~

This detects and configures the host Go SDK for use in toolchains.

If the ``GOROOT`` environment variable is set, the SDK in that directory is
used. Otherwise, ``go env GOROOT`` is used.

+--------------------------------+-----------------------------+-----------------------------------+
| **Name**                       | **Type**                    | **Default value**                 |
+--------------------------------+-----------------------------+-----------------------------------+
| :param:`name`                  | :type:`string`              | |mandatory|                       |
+--------------------------------+-----------------------------+-----------------------------------+
| A unique name for this SDK. This should almost always be :value:`go_sdk` if you want the SDK     |
| to be used by toolchains.                                                                        |
+--------------------------------+-----------------------------+-----------------------------------+
| :param:`version`               | :type:`string`              | :value:`None`                     |
+--------------------------------+-----------------------------+-----------------------------------+
| The version of Go installed on the host. If specified, `go_host_sdk` will create its repository  |
| only when the build uses a Go toolchain and `toolchain resolution`_ results in this SDK being    |
| chosen. Otherwise it will be created unconditionally.                                            |
+--------------------------------+-----------------------------+-----------------------------------+
| :param:`experiments`           | :type:`string_list`         | :value:`[]`                       |
+--------------------------------+-----------------------------+-----------------------------------+
| Go experiments to enable via `GOEXPERIMENT`.                                                     |
+--------------------------------+-----------------------------+-----------------------------------+

go_local_sdk
~~~~~~~~~~~~

This prepares a local path to use as the Go SDK in toolchains.

+--------------------------------+-----------------------------+-----------------------------------+
| **Name**                       | **Type**                    | **Default value**                 |
+--------------------------------+-----------------------------+-----------------------------------+
| :param:`name`                  | :type:`string`              | |mandatory|                       |
+--------------------------------+-----------------------------+-----------------------------------+
| A unique name for this SDK. This should almost always be :value:`go_sdk` if you want the SDK     |
| to be used by toolchains.                                                                        |
+--------------------------------+-----------------------------+-----------------------------------+
| :param:`path`                  | :type:`string`              | :value:`""`                       |
+--------------------------------+-----------------------------+-----------------------------------+
| The local path to a pre-installed Go SDK. The path must contain the go binary, the tools it      |
| invokes and the standard library sources.                                                        |
+--------------------------------+-----------------------------+-----------------------------------+
| :param:`version`               | :type:`string`              | :value:`None`                     |
+--------------------------------+-----------------------------+-----------------------------------+
| The version of the Go SDK. If specified, `go_local_sdk` will create its repository only when the |
| build uses a Go toolchain and `toolchain resolution`_ results in this SDK being chosen.          |
| Otherwise it will be created unconditionally.                                                    |
+--------------------------------+-----------------------------+-----------------------------------+
| :param:`experiments`           | :type:`string_list`         | :value:`[]`                       |
+--------------------------------+-----------------------------+-----------------------------------+
| Go experiments to enable via `GOEXPERIMENT`.                                                     |
+--------------------------------+-----------------------------+-----------------------------------+


go_wrap_sdk
~~~~~~~~~~~

This configures an SDK that was downloaded or located with another repository
rule.

+--------------------------------+-----------------------------+-----------------------------------+
| **Name**                       | **Type**                    | **Default value**                 |
+--------------------------------+-----------------------------+-----------------------------------+
| :param:`name`                  | :type:`string`              | |mandatory|                       |
+--------------------------------+-----------------------------+-----------------------------------+
| A unique name for this SDK. This should almost always be :value:`go_sdk` if you want the SDK     |
| to be used by toolchains.                                                                        |
+--------------------------------+-----------------------------+-----------------------------------+
| :param:`root_file`             | :type:`label`               | :value:`None`                     |
+--------------------------------+-----------------------------+-----------------------------------+
| A Bazel label referencing a file in the root directory of the SDK. Used to                       |
| determine the GOROOT for the SDK. This attribute and `root_files` cannot be both provided.       |
+--------------------------------+-----------------------------+-----------------------------------+
| :param:`root_files`            | :type:`string_dict`         | :value:`None`                     |
+--------------------------------+-----------------------------+-----------------------------------+
| A set of mappings from the host platform to a Bazel label referencing a file in the SDK's root   |
| directory. This attribute and `root_file` cannot be both provided.                               |
+--------------------------------+-----------------------------+-----------------------------------+
| :param:`version`               | :type:`string`              | :value:`None`                     |
+--------------------------------+-----------------------------+-----------------------------------+
| The version of the Go SDK. If specified, `go_wrap_sdk` will create its repository only when the  |
| build uses a Go toolchain and `toolchain resolution`_ results in this SDK being chosen.          |
| Otherwise it will be created unconditionally.                                                    |
+--------------------------------+-----------------------------+-----------------------------------+
| :param:`experiments`           | :type:`string_list`         | :value:`[]`                       |
+--------------------------------+-----------------------------+-----------------------------------+
| Go experiments to enable via `GOEXPERIMENT`.                                                     |
+--------------------------------+-----------------------------+-----------------------------------+


**Example:**

.. code:: bzl

    load(
        "@io_bazel_rules_go//go:deps.bzl",
        "go_register_toolchains",
        "go_rules_dependencies",
        "go_wrap_sdk",
    )

    go_wrap_sdk(
        name = "go_sdk",
        root_file = "@other_repo//go:README.md",
    )

    go_rules_dependencies()

    go_register_toolchains()

go_toolchain
~~~~~~~~~~~~

This declares a toolchain that may be used with toolchain type
:value:`"@io_bazel_rules_go//go:toolchain"`.

Normally, ``go_toolchain`` rules are declared and registered in repositories
configured with `go_download_sdk`_, `go_host_sdk`_, `go_local_sdk`_, or
`go_wrap_sdk`_. You usually won't need to declare these explicitly.

+--------------------------------+-----------------------------+-----------------------------------+
| **Name**                       | **Type**                    | **Default value**                 |
+--------------------------------+-----------------------------+-----------------------------------+
| :param:`name`                  | :type:`string`              | |mandatory|                       |
+--------------------------------+-----------------------------+-----------------------------------+
| A unique name for the toolchain.                                                                 |
+--------------------------------+-----------------------------+-----------------------------------+
| :param:`goos`                  | :type:`string`              | |mandatory|                       |
+--------------------------------+-----------------------------+-----------------------------------+
| The target operating system. Must be a standard ``GOOS`` value.                                  |
+--------------------------------+-----------------------------+-----------------------------------+
| :param:`goarch`                | :type:`string`              | |mandatory|                       |
+--------------------------------+-----------------------------+-----------------------------------+
| The target architecture. Must be a standard ``GOARCH`` value.                                    |
+--------------------------------+-----------------------------+-----------------------------------+
| :param:`sdk`                   | :type:`label`               | |mandatory|                       |
+--------------------------------+-----------------------------+-----------------------------------+
| The SDK this toolchain is based on. The target must provide `GoSDK`_. This is                    |
| usually a `go_sdk`_ rule.                                                                        |
+--------------------------------+-----------------------------+-----------------------------------+
| :param:`link_flags`            | :type:`string_list`         | :value:`[]`                       |
+--------------------------------+-----------------------------+-----------------------------------+
| Flags passed to the Go external linker.                                                          |
+--------------------------------+-----------------------------+-----------------------------------+
| :param:`cgo_link_flags`        | :type:`string_list`         | :value:`[]`                       |
+--------------------------------+-----------------------------+-----------------------------------+
| Flags passed to the external linker (if it is used).                                             |
+--------------------------------+-----------------------------+-----------------------------------+

go_context
~~~~~~~~~~

This collects the information needed to form and return a :type:`GoContext` from
a rule ctx.  It uses the attributes and the toolchains.

.. code:: bzl

  def _my_rule_impl(ctx):
      go = go_context(ctx)
      ...


+--------------------------------+-----------------------------+-----------------------------------+
| **Name**                       | **Type**                    | **Default value**                 |
+--------------------------------+-----------------------------+-----------------------------------+
| :param:`ctx`                   | :type:`ctx`                 | |mandatory|                       |
+--------------------------------+-----------------------------+-----------------------------------+
| The Bazel ctx object for the current rule.                                                       |
+--------------------------------+-----------------------------+-----------------------------------+

The context object
~~~~~~~~~~~~~~~~~~

``GoContext`` is never returned by a rule, instead you build one using
``go_context(ctx)`` in the top of any custom starlark rule that wants to interact
with the go rules.  It provides all the information needed to create go actions,
and create or interact with the other go providers.

When you get a ``GoContext`` from a context it exposes a number of fields
and methods.

All methods take the ``GoContext`` as the only positional argument. All other
arguments must be passed as keyword arguments. This allows us to re-order and
deprecate individual parameters over time.

Fields
^^^^^^

+--------------------------------+-----------------------------------------------------------------+
| **Name**                       | **Type**                                                        |
+--------------------------------+-----------------------------------------------------------------+
| :param:`toolchain`             | :type:`ToolchainInfo`                                           |
+--------------------------------+-----------------------------------------------------------------+
| The underlying toolchain. This should be considered an opaque type subject to change.            |
+--------------------------------+-----------------------------------------------------------------+
| :param:`sdk`                   | :type:`GoSDK`                                                   |
+--------------------------------+-----------------------------------------------------------------+
| The SDK in use. This may be used to access sources, packages, and tools.                         |
+--------------------------------+-----------------------------------------------------------------+
| :param:`mode`                  | :type:`Mode`                                                    |
+--------------------------------+-----------------------------------------------------------------+
| Controls the compilation setup affecting things like enabling profilers and sanitizers.          |
| See `compilation modes`_ for more information about the allowed values.                          |
+--------------------------------+-----------------------------------------------------------------+
| :param:`stdlib`                | :type:`GoStdLib`                                                |
+--------------------------------+-----------------------------------------------------------------+
| The standard library and tools to use in this build mode. This may be the                        |
| pre-compiled standard library that comes with the SDK, or it may be compiled                     |
| in a different directory for this mode.                                                          |
+--------------------------------+-----------------------------------------------------------------+
| :param:`actions`               | :type:`ctx.actions`                                             |
+--------------------------------+-----------------------------------------------------------------+
| The actions structure from the Bazel context, which has all the methods for building new         |
| bazel actions.                                                                                   |
+--------------------------------+-----------------------------------------------------------------+
| :param:`cc_toolchain_files`    | :type:`list of File`                                            |
+--------------------------------+-----------------------------------------------------------------+
| The files you need to add to the inputs of an action in order to use the cc toolchain.           |
+--------------------------------+-----------------------------------------------------------------+
| :param:`env`                   | :type:`dict of string to string`                                |
+--------------------------------+-----------------------------------------------------------------+
| Environment variables to pass to actions. Includes ``GOARCH``, ``GOOS``,                         |
| ``GOROOT``, ``GOROOT_FINAL``, ``CGO_ENABLED``, and ``PATH``.                                     |
+--------------------------------+-----------------------------------------------------------------+

Deprecated Fields
^^^^^^

+--------------------------------+-----------------------------------------------------------------+
| **Name**                       | **Type**                                                        |
+--------------------------------+-----------------------------------------------------------------+
+--------------------------------+-----------------------------------------------------------------+
| :param:`root`                  | :type:`string`                                                  |
+--------------------------------+-----------------------------------------------------------------+
| Prefer `go.env["GOROOT"]`. Path of the effective GOROOT. If :param:`stdlib` is set,              |
| this is the same as ``go.stdlib.root_file.dirname``. Otherwise, this is the same as              |
| ``go.sdk.root_file.dirname``.                                                                    |
+--------------------------------+-----------------------------------------------------------------+
| :param:`go`                    | :type:`File`                                                    |
+--------------------------------+-----------------------------------------------------------------+
| Prefer `go.sdk.go`. The main "go" binary used to run go sdk tools.                               |
+--------------------------------+-----------------------------------------------------------------+
| :param:`package_list`          | :type:`File`                                                    |
+--------------------------------+-----------------------------------------------------------------+
| Prefer `go.sdk.package_list`. A file that contains the package list of the standard library.     |
+--------------------------------+-----------------------------------------------------------------+
| :param:`tags`                  | :type:`list of string`                                          |
+--------------------------------+-----------------------------------------------------------------+
| Prefer `go.mode.tags`. List of build tags used to filter source files.                           |
+--------------------------------+-----------------------------------------------------------------+

Methods
^^^^^^^

* Action generators

  * archive_
  * binary_
  * link_

* Helpers

  * args_
  * `declare_file`_
  * `library_to_source`_
  * `new_library`_


archive
+++++++

This emits actions to compile Go code into an archive.  It supports embedding,
cgo dependencies, coverage, and assembling and packing .s files.

It returns a GoArchive_.

+--------------------------------+-----------------------------+-----------------------------------+
| **Name**                       | **Type**                    | **Default value**                 |
+--------------------------------+-----------------------------+-----------------------------------+
| :param:`go`                    | :type:`GoContext`           | |mandatory|                       |
+--------------------------------+-----------------------------+-----------------------------------+
| This must be the same GoContext object you got this function from.                               |
+--------------------------------+-----------------------------+-----------------------------------+
| :param:`source`                | :type:`GoInfo`            | |mandatory|                         |
+--------------------------------+-----------------------------+-----------------------------------+
| The GoInfo_ that should be compiled into an archive.                                             |
+--------------------------------+-----------------------------+-----------------------------------+


binary
++++++

This emits actions to compile and link Go code into a binary.  It supports
embedding, cgo dependencies, coverage, and assembling and packing .s files.

It returns a tuple containing GoArchive_, the output executable file, and
a ``runfiles`` object.

+--------------------------------+-----------------------------+-----------------------------------+
| **Name**                       | **Type**                    | **Default value**                 |
+--------------------------------+-----------------------------+-----------------------------------+
| :param:`go`                    | :type:`GoContext`           | |mandatory|                       |
+--------------------------------+-----------------------------+-----------------------------------+
| This must be the same GoContext object you got this function from.                               |
+--------------------------------+-----------------------------+-----------------------------------+
| :param:`name`                  | :type:`string`              | :value:`""`                       |
+--------------------------------+-----------------------------+-----------------------------------+
| The base name of the generated binaries. Required if :param:`executable` is not given.           |
+--------------------------------+-----------------------------+-----------------------------------+
| :param:`source`                | :type:`GoInfo`            | |mandatory|                         |
+--------------------------------+-----------------------------+-----------------------------------+
| The GoInfo_ that should be compiled and linked.                                                  |
+--------------------------------+-----------------------------+-----------------------------------+
| :param:`test_archives`         | :type:`list GoArchiveData`  | :value:`[]`                       |
+--------------------------------+-----------------------------+-----------------------------------+
| List of archives for libraries under test. See link_.                                            |
+--------------------------------+-----------------------------+-----------------------------------+
| :param:`gc_linkopts`           | :type:`string_list`         | :value:`[]`                       |
+--------------------------------+-----------------------------+-----------------------------------+
| Go link options.                                                                                 |
+--------------------------------+-----------------------------+-----------------------------------+
| :param:`version_file`          | :type:`File`                | :value:`None`                     |
+--------------------------------+-----------------------------+-----------------------------------+
| Version file used for link stamping. See link_.                                                  |
+--------------------------------+-----------------------------+-----------------------------------+
| :param:`info_file`             | :type:`File`                | :value:`None`                     |
+--------------------------------+-----------------------------+-----------------------------------+
| Info file used for link stamping. See link_.                                                     |
+--------------------------------+-----------------------------+-----------------------------------+
| :param:`executable`            | :type:`File`                | :value:`None`                     |
+--------------------------------+-----------------------------+-----------------------------------+
| Optional output file to write. If not set, ``binary`` will generate an output                    |
| file name based on ``name``, the target platform, and the link mode.                             |
+--------------------------------+-----------------------------+-----------------------------------+


link
++++

The link function adds an action that runs ``go tool link`` on a library.

It does not return anything.

+--------------------------------+-----------------------------+-----------------------------------+
| **Name**                       | **Type**                    | **Default value**                 |
+--------------------------------+-----------------------------+-----------------------------------+
| :param:`go`                    | :type:`GoContext`           | |mandatory|                       |
+--------------------------------+-----------------------------+-----------------------------------+
| This must be the same GoContext object you got this function from.                               |
+--------------------------------+-----------------------------+-----------------------------------+
| :param:`archive`               | :type:`GoArchive`           | |mandatory|                       |
+--------------------------------+-----------------------------+-----------------------------------+
| The library to link.                                                                             |
+--------------------------------+-----------------------------+-----------------------------------+
| :param:`test_archives`         | :type:`GoArchiveData list`  | :value:`[]`                       |
+--------------------------------+-----------------------------+-----------------------------------+
| List of archives for libraries under test. These are excluded from linking                       |
| if transitive dependencies of :param:`archive` have the same package paths.                      |
| This is useful for linking external test archives that depend internal test                      |
| archives.                                                                                        |
+--------------------------------+-----------------------------+-----------------------------------+
| :param:`executable`            | :type:`File`                | |mandatory|                       |
+--------------------------------+-----------------------------+-----------------------------------+
| The binary to produce.                                                                           |
+--------------------------------+-----------------------------+-----------------------------------+
| :param:`gc_linkopts`           | :type:`string_list`         | :value:`[]`                       |
+--------------------------------+-----------------------------+-----------------------------------+
| Basic link options, these may be adjusted by the :param:`mode`.                                  |
+--------------------------------+-----------------------------+-----------------------------------+
| :param:`version_file`          | :type:`File`                | :value:`None`                     |
+--------------------------------+-----------------------------+-----------------------------------+
| Version file used for link stamping.                                                             |
+--------------------------------+-----------------------------+-----------------------------------+
| :param:`info_file`             | :type:`File`                | :value:`None`                     |
+--------------------------------+-----------------------------+-----------------------------------+
| Info file used for link stamping.                                                                |
+--------------------------------+-----------------------------+-----------------------------------+


args
++++

This creates a new Args_ object, using the ``ctx.actions.args`` method. The
object is pre-populated with standard arguments used by all the go toolchain
builders.

+--------------------------------+-----------------------------+-----------------------------------+
| **Name**                       | **Type**                    | **Default value**                 |
+--------------------------------+-----------------------------+-----------------------------------+
| :param:`go`                    | :type:`GoContext`           | |mandatory|                       |
+--------------------------------+-----------------------------+-----------------------------------+
| This must be the same GoContext object you got this function from.                               |
+--------------------------------+-----------------------------+-----------------------------------+

declare_file
++++++++++++

This is the equivalent of ``ctx.actions.declare_file``. It uses the
current build mode to make the filename unique between configurations.

+--------------------------------+-----------------------------+-----------------------------------+
| **Name**                       | **Type**                    | **Default value**                 |
+--------------------------------+-----------------------------+-----------------------------------+
| :param:`go`                    | :type:`GoContext`           | |mandatory|                       |
+--------------------------------+-----------------------------+-----------------------------------+
| This must be the same GoContext object you got this function from.                               |
+--------------------------------+-----------------------------+-----------------------------------+
| :param:`path`                  | :type:`string`              | :value:`""`                       |
+--------------------------------+-----------------------------+-----------------------------------+
| A path for this file, including the basename of the file.                                        |
+--------------------------------+-----------------------------+-----------------------------------+
| :param:`ext`                   | :type:`string`              | :value:`""`                       |
+--------------------------------+-----------------------------+-----------------------------------+
| The extension to use for the file.                                                               |
+--------------------------------+-----------------------------+-----------------------------------+
| :param:`name`                  | :type:`string`              | :value:`""`                       |
+--------------------------------+-----------------------------+-----------------------------------+
| A name to use for this file. If path is not present, this becomes a prefix to the path.          |
| If this is not set, the current rule name is used in it's place.                                 |
+--------------------------------+-----------------------------+-----------------------------------+

new_go_info
+++++++++++++++++

This is used to build a GoInfo object in the current build mode.

+--------------------------------+-----------------------------+-----------------------------------+
| **Name**                       | **Type**                    | **Default value**                 |
+--------------------------------+-----------------------------+-----------------------------------+
| :param:`go`                    | :type:`GoContext`           | |mandatory|                       |
+--------------------------------+-----------------------------+-----------------------------------+
| The GoContext object for this target.                                                            |
+--------------------------------+-----------------------------+-----------------------------------+
| :param:`attr`                  | :type:`ctx.attr`            | |mandatory|                       |
+--------------------------------+-----------------------------+-----------------------------------+
| The attributes of the target being analyzed. For most rules, this should be                      |
| ``ctx.attr``. Rules can also pass in a ``struct`` with the same fields.                          |
|                                                                                                  |
| ``library_to_source`` looks for fields corresponding to the attributes of                        |
| ``go_library`` and ``go_binary``. This includes ``srcs``, ``deps``, ``embed``,                   |
| and so on. All fields are optional (and may not be defined in the struct                         |
| argument at all), but if they are present, they must have the same types and                     |
| allowed values as in ``go_library`` and ``go_binary``. For example, ``srcs``                     |
| must be a list of ``Targets``; ``gc_goopts`` must be a list of strings.                          |
|                                                                                                  |
| As an exception, ``deps``, if present, must be a list containing either                          |
| ``Targets`` or ``GoArchives``.                                                                   |
+--------------------------------+-----------------------------+-----------------------------------+
| :param:`name`                  | :type:`string`                                                  |
+--------------------------------+-----------------------------------------------------------------+
| The name of the library. Usually, this is the ``name`` attribute.                                |
+--------------------------------+-----------------------------------------------------------------+
| :param:`importpath`            | :type:`string`                                                  |
+--------------------------------+-----------------------------------------------------------------+
| The string used in ``import`` declarations in Go source code to import                           |
| this library. Usually, this is the ``importpath`` attribute.                                     |
+--------------------------------+-----------------------------+-----------------------------------+
| :param:`resolver`              | :type:`function`            | :value:`None`                     |
+--------------------------------+-----------------------------+-----------------------------------+
| This is the function that gets invoked when building the GoInfo.                                 |
| The function's signature must be                                                                 |
|                                                                                                  |
| .. code:: bzl                                                                                    |
|                                                                                                  |
|     def _stdlib_library_to_source(go, attr, source, merge)                                       |
|                                                                                                  |
| attr is the attributes of the rule being processed                                               |
| source is the dictionary of GoInfo fields being generated                                        |
| merge is a helper you can call to merge                                                          |
+--------------------------------+-----------------------------+-----------------------------------+
| :param:`importable`            | :type:`bool`                | |False|                           |
+--------------------------------+-----------------------------+-----------------------------------+
| This controls whether the GoInfo_ is supposed to be importable. This is generally only false     |
| for the "main" libraries that are built just before linking.                                     |
+--------------------------------+-----------------------------+-----------------------------------+
| :param:`testfilter`            | :type:`string`              | |None|                            |
+--------------------------------+-----------------------------+-----------------------------------+
| :param:`is_main`               | :type:`bool`                                                    |
+--------------------------------+-----------------------------------------------------------------+
| Indicates whether the library should be compiled as a `main` package.                            |
| `main` packages may have arbitrary `importpath` and `importmap` values,                          |
| but the compiler and linker must see them as `main`.                                             |
+--------------------------------+-----------------------------+-----------------------------------+
| :param:`coverage_instrumented` | :type:`bool`                | |None|                            |
+--------------------------------+-----------------------------+-----------------------------------+
| This controls whether cover is enabled for this specific library in this mode.                   |
| If ommitted, it falls back to ctx.coverage_instrumented()                                        |
+--------------------------------+-----------------------------+-----------------------------------+
| :param:`generated_srcs`        | :type:`List[file]`          | |None|                            |
+--------------------------------+-----------------------------+-----------------------------------+
| :param:`pathtype`              | :type:`string`                                                  |
+--------------------------------+-----------------------------------------------------------------+
| Information about the source of the importpath. Possible values are:                             |
|                                                                                                  |
| :value:`explicit`                                                                                |
|     The importpath was explicitly supplied by the user and the library is importable.            |
|     This is the normal case.                                                                     |
| :value:`inferred`                                                                                |
|     The importpath was inferred from the directory structure and rule name. The library may be   |
|     importable.                                                                                  |
|     This is normally true for rules that do not expect to be compiled directly to a library,     |
|     embeded into another rule instead (source generators)                                        |
| :value:`export`                                                                                  |
|     The importpath was explicitly supplied by the user, but the library is                       |
|     not importable. This is the case for binaries and tests. The importpath                      |
|     may still be useful for `go_path`_ and other rules.                                          |
+--------------------------------+-----------------------------+-----------------------------------+
