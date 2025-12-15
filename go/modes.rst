Build modes
===========

.. _Bazel build settings: https://docs.bazel.build/versions/master/skylark/config.html#using-build-settings
.. _Bazel configuration transitions: https://docs.bazel.build/versions/master/skylark/lib/transition.html
.. _Bazel platform: https://docs.bazel.build/versions/master/platforms.html

.. _go_library: /docs/go/core/rules.md#go_library
.. _go_binary: /docs/go/core/rules.md#go_binary
.. _go_test: /docs/go/core/rules.md#go_test
.. _toolchain: toolchains.rst#the-toolchain-object

.. _config_setting: https://docs.bazel.build/versions/master/be/general.html#config_setting
.. _platform: https://docs.bazel.build/versions/master/be/platform.html#platform
.. _select: https://docs.bazel.build/versions/master/be/functions.html#select

.. role:: param(kbd)
.. role:: type(emphasis)
.. role:: value(code)

.. contents:: :depth: 2

Overview
--------

The Go toolchain can be configured to build targets in different modes using
`Bazel build settings`_ specified on the command line or by using attributes
specified on individual `go_binary`_ or `go_test`_ targets. For example, tests
may be run in race mode with the command line flag
``--@io_bazel_rules_go//go/config:race`` or by setting ``race = "on"`` on the
individual test targets.

Similarly, the Go toolchain can be made to cross-compile binaries for a specific
platform by setting the ``--platforms`` command line flag or by setting the
``goos`` and ``goarch`` attributes of the binary target. For example, a binary
could be built for ``linux`` / ``arm64`` using the command line flag
``--platforms=@io_bazel_rules_go//go/toolchain:linux_arm64`` or by setting
``goos = "linux"`` and ``goarch = "arm64"``.

Build settings
--------------

The build settings below are defined in the package
``@io_bazel_rules_go//go/config``. They can all be set on the command line
or using `Bazel configuration transitions`_.

+------------------------+---------------------+-------------------------------+
| **Name**               | **Type**            | **Default value**             |
+------------------------+---------------------+-------------------------------+
| :param:`static`        | :type:`bool`        | :value:`false`                |
+------------------------+---------------------+-------------------------------+
| Statically links the target binary. May not always work since parts of the   |
| standard library and other C dependencies won't tolerate static linking.     |
| Works best with ``pure`` set as well.                                        |
+------------------------+---------------------+-------------------------------+
| :param:`race`          | :type:`bool`        | :value:`false`                |
+------------------------+---------------------+-------------------------------+
| Instruments the binary for race detection. Programs will panic when a data   |
| race is detected. Requires cgo. Mutually exclusive with ``msan``.            |
+------------------------+---------------------+-------------------------------+
| :param:`msan`          | :type:`bool`        | :value:`false`                |
+------------------------+---------------------+-------------------------------+
| Instruments the binary for memory sanitization. Requires cgo. Mutually       |
| exclusive with ``race``.                                                     |
+------------------------+---------------------+-------------------------------+
| :param:`pure`          | :type:`bool`        | :value:`false`                |
+------------------------+---------------------+-------------------------------+
| Disables cgo, even when a C/C++ toolchain is configured (similar to setting  |
| ``CGO_ENABLED=0``). Packages that contain cgo code may still be built, but   |
| the cgo code will be filtered out, and the ``cgo`` build tag will be false.  |
+------------------------+---------------------+-------------------------------+
| :param:`debug`         | :type:`bool`        | :value:`false`                |
+------------------------+---------------------+-------------------------------+
| Includes debugging information in compiled packages (using the ``-N`` and    |
| ``-l`` flags). This is always true with ``-c dbg``.                          |
+------------------------+---------------------+-------------------------------+
| :param:`gotags`        | :type:`string_list` | :value:`[]`                   |
+------------------------+---------------------+-------------------------------+
| Controls which build tags are enabled when evaluating build constraints in   |
| source files. Useful for conditional compilation.                            |
+------------------------+---------------------+-------------------------------+
| :param:`linkmode`      | :type:`string`      | :value:`"auto"`               |
+------------------------+---------------------+-------------------------------+
| Determines how the Go binary is built and linked. Similar to ``-buildmode``. |
| Must be one of ``"normal"``, ``"shared"``, ``"pie"``, ``"plugin"``,          |
| ``"c-shared"``, ``"c-archive"``.                                             |
|                                                                              |
| When left at the default ``"auto"``, this builds PIE binaries on Android,    |
| iOS, macOS and Windows (unless ``--@io_bazel_rules_go//go/config:race`` is   |
| enabled) and uses ``"normal"`` elsewhere. Set ``"normal"`` explicitly if you |
| need position-dependent binaries on a platform that would otherwise use PIE. |
+------------------------+---------------------+-------------------------------+
| :param:`export_stdlib` | :type:`bool`        | :value:`false`                |
+------------------------+---------------------+-------------------------------+
| This controls whether exports for the stdlib are generated by rules_go.      |
| This is useful for running tools like golintci-lint via GOPACKAGESDRIVER     |
| but adds time to the initial build. Leave false unless you want to use       |
| golangci-lint or another tool that relies on GOPACKAGESDRIVER.               |
+------------------------+---------------------+-------------------------------+

Platforms
---------

You can define a `Bazel platform`_ using the native `platform`_ rule. A platform
is essentially a list of facts (constraint values) about a target platform.
rules_go defines a ``platform`` for each configuration the Go toolchain supports
in ``@io_bazel_rules_go//go/toolchain``. There are also `config_setting`_ targets
in ``@io_bazel_rules_go//go/platform`` that can be used to pick platform-specific
sources or dependencies using `select`_.

You can specify a target platform using the ``--platforms`` command line flag.
Bazel will automatically select a registered toolchain compatible with the
target platform (rules_go registers toolchains for all supported platforms).
For example, you could build for Linux / arm64 with the flag
``--platforms=@io_bazel_rules_go//go/toolchain:linux_arm64``.

You can set the ``goos`` and ``goarch`` attributes on an individual
`go_binary`_ or `go_test`_ rule to build a binary for a specific platform.
This sets the ``--platforms`` flag via `Bazel configuration transitions`_.


Examples
--------

Building pure go binaries
~~~~~~~~~~~~~~~~~~~~~~~~~

You can switch the default binaries to non cgo using

.. code:: bash
    bazel build --@io_bazel_rules_go//go/config:pure //:my_binary
You can build pure go binaries by setting those attributes on a binary.

.. code:: bzl

    go_binary(
        name = "foo",
        srcs = ["foo.go"],
        pure = "on",
    )


Building static binaries
~~~~~~~~~~~~~~~~~~~~~~~~

| Note that static linking does not work on darwin.

You can switch the default binaries to statically linked binaries using

.. code:: bash
    bazel build --@io_bazel_rules_go//go/config:static //:my_binary
You can build static go binaries by setting those attributes on a binary.
If you want it to be fully static (no libc), you should also specify pure.

.. code:: bzl

    go_binary(
        name = "foo",
        srcs = ["foo.go"],
        static = "on",
    )


Using the race detector
~~~~~~~~~~~~~~~~~~~~~~~

You can switch the default binaries to race detection mode, and thus also switch
the mode of tests by using

.. code::

    bazel test --@io_bazel_rules_go//go/config:race //...

Alternatively, you can activate race detection for specific tests.

.. code::

    go_test(
        name = "go_default_test",
        srcs = ["lib_test.go"],
        embed = [":go_default_library"],
        race = "on",
  )
