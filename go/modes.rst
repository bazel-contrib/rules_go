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
| :param:`internal_link` | :type:`bool`        | :value:`false`                |
+------------------------+---------------------+-------------------------------+
| When set, a `go_binary`_ or `go_test`_ with no cgo in its dependencies links |
| with the Go linker. Its link action gets no C/C++ toolchain files or         |
| environment. The setting also applies to Go tools that Bazel builds for the  |
| execution platform. On Linux, this needs the netgo and osusergo build tags,  |
| so that the standard library has no cgo. A binary that imports the plugin    |
| package must set ``pure = "off"``, because rules_go cannot detect that       |
| import. See `Choosing the linker`_.                                          |
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

Choosing the linker
-------------------

The Go linker can link a binary by itself, or it can run the C/C++ linker. Go
calls these modes internal linking and external linking.

When the Go linker chooses
~~~~~~~~~~~~~~~~~~~~~~~~~~

rules_go lets the Go linker choose the link mode for a `go_binary`_ or
`go_test`_ when all of these conditions are true:

* No package in the dependencies of the binary uses cgo or contains a
  ``.syso`` file.
* The standard library does not use cgo. On Unix systems other than macOS and
  iOS, the net package uses cgo unless you set the ``netgo`` build tag.
* On the same systems, the os/user package uses cgo unless you set the
  ``osusergo`` build tag. Android is an exception.
* With the ``boringcrypto`` Go experiment on Linux amd64 or arm64, the crypto
  packages use cgo.
* The link mode is ``normal``. On macOS and Windows, the ``pie`` link mode is
  also possible.
* Race and msan instrumentation are off.
* The Go linker flags in ``gc_linkopts`` and in the Go toolchain do not select
  the external linker or a link mode. One example of such a flag is
  ``-linkmode=external``.
* The build has no ``--linkopt`` flag. Its values are flags for the C/C++
  linker.
* If ``static`` is on, the target is not AIX, illumos, Solaris, or OpenBSD.
  On these systems, the ``syscall`` package links libc dynamically.
* The Go SDK is version 1.24 or later.

To make the standard library free of cgo on Linux, build with
``--define=gotags=netgo,osusergo``. You can also set
``gotags = ["netgo", "osusergo"]`` on the binary.

The Go linker then decides from the objects that it links, the same as the go
command. It links a pure Go binary internally. It uses the external linker
when an object needs it. One example is a binary that imports the plugin
package. The link step still gets the C/C++ toolchain files and environment
variables, because the Go linker can select the external linker.

These are the effects:

* On macOS and Windows, the default ``pie`` link mode does not force the
  external linker. The Go linker links a pure Go binary internally.
* With ``static`` on Linux, FreeBSD, NetBSD, and DragonFly BSD, the Go linker
  links a pure Go binary internally. The result is a static binary.
* On macOS, ``static`` has no effect when the Go linker links internally.
  macOS does not support static binaries.
* On Linux in normal mode without ``static``, nothing changes.

If a condition in the list is false, rules_go keeps its own rules. It forces
the external linker when ``static`` is on, or when race instrumentation is on.
It also forces it for a link mode other than normal, and for msan
instrumentation on Windows. In the other cases, the Go linker chooses.

Linking without the C/C++ toolchain
~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~

Set ``--@io_bazel_rules_go//go/config:internal_link`` to always use the Go
linker for a binary that meets the conditions in the list. rules_go then
passes ``-linkmode internal`` to the Go linker. The link step gets no C/C++
toolchain files, flags, or environment variables. The build information of
the binary still records ``CGO_ENABLED=1``, the same as the go command.

The flag also applies to Go tools that Bazel builds for the execution
platform. For these tools on Linux, set the build tags with
``--define=gotags=netgo,osusergo``. Newer Bazel versions do not pass the tags
setting of rules_go to the execution platform.

rules_go cannot see which standard library packages a binary imports. The
plugin package uses cgo and needs the external linker. With this flag, a
binary that imports the plugin package must set ``pure = "off"``. If it does
not, the binary stops with a fatal error when it calls ``plugin.Open``.

In ``pure`` mode, rules_go always links internally.

How to opt out
~~~~~~~~~~~~~~

These settings make a condition in the list false. rules_go then uses its own
rules from the end of `When the Go linker chooses`_. The settings work with
and without the ``internal_link`` flag.

* For one target, set ``pure = "off"``.
* For one target, add ``-linkmode=external`` to the gc_linkopts attribute.
  This setting always selects the external linker.
* For all targets, build with
  ``--@io_bazel_rules_go//go/config:gc_linkopts=-linkmode=external``.

Limitations
~~~~~~~~~~~

* With Go versions before 1.27 on macOS, the Go linker writes 12.0 as the
  minimum macOS version. It ignores ``--macos_minimum_os``. The ``go build``
  command does the same.


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

| macOS does not support static binaries. rules_go ignores ``static`` on macOS
  when the Go linker links the binary internally. Otherwise, the link fails.

You can switch the default binaries to statically linked binaries using

.. code:: bash
    bazel build --@io_bazel_rules_go//go/config:static //:my_binary
You can build static go binaries by setting those attributes on a binary.
A binary with no libc code must not use cgo. Use pure mode, or meet the
conditions in `Choosing the linker`_.

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
