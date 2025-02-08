Go providers
============

.. _providers: https://docs.bazel.build/versions/master/skylark/rules.html#providers

.. _go_library: /docs/go/core/rules.md#go_library
.. _go_binary: /docs/go/core/rules.md#go_binary
.. _go_test: /docs/go/core/rules.md#go_test
.. _go_path: /docs/go/core/rules.md#go_path
.. _cc_library: https://docs.bazel.build/versions/master/be/c-cpp.html#cc_library
.. _flatbuffers: http://google.github.io/flatbuffers/
.. _static linking: modes.rst#building-static-binaries
.. _race detector: modes.rst#using-the-race-detector
.. _runfiles: https://docs.bazel.build/versions/master/skylark/lib/runfiles.html
.. _File: https://docs.bazel.build/versions/master/skylark/lib/File.html
.. _new_go_info: toolchains.rst#new_go_info
.. _archive: toolchains.rst#archive

.. role:: param(kbd)
.. role:: type(emphasis)
.. role:: value(code)
.. |mandatory| replace:: **mandatory value**


The providers_ are the outputs of the rules. You generaly get them by having a
dependency on a rule, and then asking for a provider of a specific type.

.. contents:: :depth: 2

-----

Design
------

The Go providers are designed primarily for the efficiency of the Go rules. The
information they share is mostly there because it is required for the core rules
to work.

All the providers are designed to hold only immutable data. This is partly
because its a cleaner design choice to be able to assume a provider will never
change, but also because only immutable objects are allowed to be stored in a
depset, and it's really useful to have depsets of providers.  Specifically the
:param:`direct` and :param:`transitive` fields on GoArchive_ only work because
GoArchiveData_ is immutable.

API
---

GoInfo
~~~~~~~~

GoInfo contains metadata about an individual library.
It takes into account mode-specific processing, ready to build
a GoArchive_. This is produced by calling the `new_go_info`_ helper
method. In general, only rules_go should need to build or handle these.

+--------------------------------+-----------------------------------------------------------------+
| **Name**                       | **Type**                                                        |
+--------------------------------+-----------------------------------------------------------------+
| :param:`name`                  | :type:`string`                                                  |
+--------------------------------+-----------------------------------------------------------------+
| The name of the library. Usually, this is the ``name`` attribute.                                |
+--------------------------------+-----------------------------------------------------------------+
| :param:`label`                 | :type:`Label`                                                   |
+--------------------------------+-----------------------------------------------------------------+
| The full label for the library.                                                                  |
+--------------------------------+-----------------------------------------------------------------+
| :param:`importpath`            | :type:`string`                                                  |
+--------------------------------+-----------------------------------------------------------------+
| The string used in ``import`` declarations in Go source code to import                           |
| this library. Usually, this is the ``importpath`` attribute.                                     |
+--------------------------------+-----------------------------------------------------------------+
| :param:`importmap`             | :type:`string`                                                  |
+--------------------------------+-----------------------------------------------------------------+
| The package path for this library. The Go compiler and linker internally refer                   |
| to the library using this string. It must be unique in any binary the library                    |
| is linked into. This is usually the same as ``importpath``, but it may be                        |
| different, especially for vendored libraries.                                                    |
+--------------------------------+-----------------------------------------------------------------+
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
+--------------------------------+-----------------------------------------------------------------+
| :param:`is_main`               | :type:`bool`                                                    |
+--------------------------------+-----------------------------------------------------------------+
| Indicates whether the library should be compiled as a `main` package.                            |
| `main` packages may have arbitrary `importpath` and `importmap` values,                          |
| but the compiler and linker must see them as `main`.                                             |
+--------------------------------+-----------------------------------------------------------------+
| :param:`mode`                  | :type:`Mode`                                                    |
+--------------------------------+-----------------------------------------------------------------+
| The mode this library is being built for.                                                        |
+--------------------------------+-----------------------------------------------------------------+
| :param:`srcs`                  | :type:`list of File`                                            |
+--------------------------------+-----------------------------------------------------------------+
| The sources to compile into the archive.                                                         |
+--------------------------------+-----------------------------------------------------------------+
| :param:`embedsrcs`             | :type:`list of File`                                            |
+--------------------------------+-----------------------------------------------------------------+
| Files that may be embedded into the compiled package using ``//go:embed``                        |
| directives. All files must be in the same logical directory or a subdirectory                    |
| as source files. However, it's okay to mix static and generated source files                     |
| and static and generated embeddable files.                                                       |
+--------------------------------+-----------------------------------------------------------------+
| :param:`cover`                 | :type:`list of File`                                            |
+--------------------------------+-----------------------------------------------------------------+
| List of source files to instrument for code coverage.                                            |
+--------------------------------+-----------------------------------------------------------------+
| :param:`x_defs`                | :type:`string_dict`                                             |
+--------------------------------+-----------------------------------------------------------------+
| Map of defines to add to the go link command.                                                    |
+--------------------------------+-----------------------------------------------------------------+
| :param:`deps`                  | :type:`list of GoArchive`                                       |
+--------------------------------+-----------------------------------------------------------------+
| The direct dependencies needed by this library.                                                  |
+--------------------------------+-----------------------------------------------------------------+
| :param:`gc_goopts`             | :type:`list of string`                                          |
+--------------------------------+-----------------------------------------------------------------+
| Go compilation options that should be used when compiling these sources.                         |
| In general these will be used for *all* sources of any library this provider is embedded into.   |
+--------------------------------+-----------------------------------------------------------------+
| :param:`runfiles`              | :type:`Runfiles`                                                |
+--------------------------------+-----------------------------------------------------------------+
| The set of files needed by code in these sources at runtime.                                     |
+--------------------------------+-----------------------------------------------------------------+
| :param:`cgo`                   | :type:`bool`                                                    |
+--------------------------------+-----------------------------------------------------------------+
| True if the library may contain cgo sources or C/C++/ObjC sources.                               |
| If true and cgo is enabled, cgo sources will be processed with cgo, and                          |
| C/C++/ObjC will be compiled with the appropriate toolchain and packed into                       |
| the final archive. If true and cgo is disabled, cgo sources are filtered                         |
| out, and sources with ``// +build !cgo`` are included.                                           |
+--------------------------------+-----------------------------------------------------------------+
| :param:`cdeps`                 | :type:`list of Target`                                          |
+--------------------------------+-----------------------------------------------------------------+
| List of ``cc_library`` and ``objc_library`` targets this library depends on.                     |
+--------------------------------+-----------------------------------------------------------------+
| :param:`cppopts`               | :type:`list of string`                                          |
+--------------------------------+-----------------------------------------------------------------+
| List of additional flags to pass to the C preprocessor when invoking the                         |
| C/C++/ObjC compilers.                                                                            |
+--------------------------------+-----------------------------------------------------------------+
| :param:`copts`                 | :type:`list of string`                                          |
+--------------------------------+-----------------------------------------------------------------+
| List of additional flags to pass to the C compiler.                                              |
+--------------------------------+-----------------------------------------------------------------+
| :param:`cxxopts`               | :type:`list of string`                                          |
+--------------------------------+-----------------------------------------------------------------+
| List of additional flags to pass to the C++ compiler.                                            |
+--------------------------------+-----------------------------------------------------------------+
| :param:`clinkopts`             | :type:`list of string`                                          |
+--------------------------------+-----------------------------------------------------------------+
| List of additional flags to pass to the external linker.                                         |
+--------------------------------+-----------------------------------------------------------------+
| :param:`cc_info`               | :type:`CcInfo`                                                  |
+--------------------------------+-----------------------------------------------------------------+
| The result of merging the ``CcInfo``s of all `deps` and `cdeps`                                  |
+--------------------------------+-----------------------------------------------------------------+

GoArchiveData
~~~~~~~~~~~~~

GoArchiveData contains information about a compiled Go package. GoArchiveData
only contains immutable information about a package itself. It does not contain
any information about dependencies or references to other providers. This makes
it suitable to include in depsets. GoArchiveData is not directly returned by any
rule.  Instead, it's referenced in the ``data`` field of GoArchive_.

+--------------------------------+-----------------------------------------------------------------+
| **Name**                       | **Type**                                                        |
+--------------------------------+-----------------------------------------------------------------+
| :param:`name`                  | :type:`string`                                                  |
+--------------------------------+-----------------------------------------------------------------+
| The name of the library. Usually the same as the ``name`` attribute.                             |
+--------------------------------+-----------------------------------------------------------------+
| :param:`label`                 | :type:`Label`                                                   |
+--------------------------------+-----------------------------------------------------------------+
| The full label for the library.                                                                  |
+--------------------------------+-----------------------------------------------------------------+
| :param:`importpath`            | :type:`string`                                                  |
+--------------------------------+-----------------------------------------------------------------+
| The string used in ``import`` declarations in Go source code to import this                      |
| library. Usually, this is the ``importpath`` attribute.                                          |
+--------------------------------+-----------------------------------------------------------------+
| :param:`importmap`             | :type:`string`                                                  |
+--------------------------------+-----------------------------------------------------------------+
| The package path for this library. The Go compiler and linker internally refer                   |
| to the library using this string. It must be unique in any binary the library                    |
| is linked into. This is usually the same as ``importpath``, but it may be                        |
| different, especially for vendored libraries.                                                    |
+--------------------------------+-----------------------------------------------------------------+
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
+--------------------------------+-----------------------------------------------------------------+
| :param:`file`                  | :type:`File`                                                    |
+--------------------------------+-----------------------------------------------------------------+
| The archive file for the linker produced when this library is compiled.                          |
+--------------------------------+-----------------------------------------------------------------+
| :param:`export_file`           | :type:`File`                                                    |
+--------------------------------+-----------------------------------------------------------------+
| The archive file for compilation of dependent libraries produced when this library is compiled.  |
+--------------------------------+-----------------------------------------------------------------+
| :param:`facts_file`            | :type:`File`                                                    |
+--------------------------------+-----------------------------------------------------------------+
| The serialized facts for this library produced when nogo ran for this library.                   |
+--------------------------------+-----------------------------------------------------------------+
| :param:`srcs`                  | :type:`tuple of File`                                           |
+--------------------------------+-----------------------------------------------------------------+
| The .go sources compiled into the archive. May have been generated or                            |
| transformed with tools like cgo and cover.                                                       |
+--------------------------------+-----------------------------------------------------------------+
| :param:`runfiles`              | :type:`runfiles`                                                |
+--------------------------------+-----------------------------------------------------------------+
| Data files that should be available at runtime to binaries and tests built                       |
| from this archive.                                                                               |
+--------------------------------+-----------------------------------------------------------------+

GoArchive
~~~~~~~~~

``GoArchive`` contains information about a compiled archive and its dependencies
(both direct and transitive). This is used when compiling and linking Go
libraries and binaries. It is produced by the archive_ toolchain function.

Most of the metadata about the archive itself is available in GoArchiveData_,
which is available through the :param:`data` field.

+--------------------------------+-----------------------------------------------------------------+
| **Name**                       | **Type**                                                        |
+--------------------------------+-----------------------------------------------------------------+
| :param:`source`                | :type:`GoInfo`                                                |
+--------------------------------+-----------------------------------------------------------------+
| The source provider this GoArchive was compiled from.                                            |
+--------------------------------+-----------------------------------------------------------------+
| :param:`data`                  | :type:`GoArchiveData`                                           |
+--------------------------------+-----------------------------------------------------------------+
| The non transitive data for this archive.                                                        |
+--------------------------------+-----------------------------------------------------------------+
| :param:`direct`                | :type:`list of GoArchive`                                       |
+--------------------------------+-----------------------------------------------------------------+
| The direct dependencies of this archive.                                                         |
+--------------------------------+-----------------------------------------------------------------+
| :param:`libs`                  | :type:`depset of File`                                          |
+--------------------------------+-----------------------------------------------------------------+
| The transitive set of libraries needed to link with this archive.                                |
+--------------------------------+-----------------------------------------------------------------+
| :param:`transitive`            | :type:`depset of GoArchiveData`                                 |
+--------------------------------+-----------------------------------------------------------------+
| The full set of transitive dependencies. This includes ``data`` for this                         |
| archive and all ``data`` members transitively reachable through ``direct``.                      |
+--------------------------------+-----------------------------------------------------------------+
| :param:`x_defs`                | :type:`string_dict`                                             |
+--------------------------------+-----------------------------------------------------------------+
| The full transitive set of defines to add to the go link command.                                |
+--------------------------------+-----------------------------------------------------------------+
| :param:`cgo_deps`              | :type:`depset(cc_library)`                                      |
+--------------------------------+-----------------------------------------------------------------+
| The direct cgo dependencies of this library.                                                     |
| This has the same constraints as things that can appear in the deps of a cc_library_.            |
+--------------------------------+-----------------------------------------------------------------+
| :param:`cgo_exports`           | :type:`depset of GoInfo`                                        |
+--------------------------------+-----------------------------------------------------------------+
| The transitive set of c headers needed to reference exports of this archive.                     |
+--------------------------------+-----------------------------------------------------------------+
| :param:`runfiles`              | runfiles_                                                       |
+--------------------------------+-----------------------------------------------------------------+
| The files needed to run anything that includes this library.                                     |
+--------------------------------+-----------------------------------------------------------------+

GoPath
~~~~~~

GoPath is produced by the `go_path`_ rule. It gives a list of packages used to
build the ``go_path`` directory and provides a list of original files for each
package.

+--------------------------------+-----------------------------------------------------------------+
| **Name**                       | **Type**                                                        |
+--------------------------------+-----------------------------------------------------------------+
| :param:`gopath`                | :type:`string`                                                  |
+--------------------------------+-----------------------------------------------------------------+
| The short path to the output file or directory. Useful for constructing                          |
| ``runfiles`` paths.                                                                              |
+--------------------------------+-----------------------------------------------------------------+
| :param:`gopath_file`           | :type:`File`                                                    |
+--------------------------------+-----------------------------------------------------------------+
| A Bazel File_ that points to the output directory.                                               |
|                                                                                                  |
| * In ``archive`` mode, this is the archive.                                                      |
| * In ``copy`` mode, this is the output directory.                                                |
| * In ``link`` mode, this is an empty file inside the output directory, so                        |
|   you need to use .dirname to get the path to the directory.                                     |
+--------------------------------+-----------------------------------------------------------------+
| :param:`packages`              | :type:`list of struct`                                          |
+--------------------------------+-----------------------------------------------------------------+
| A list of structs representing packages used to build the ``go_path``                            |
| directory. Each struct has the following fields:                                                 |
|                                                                                                  |
| * ``importpath``: the import path of the package.                                                |
| * ``dir``: the subdirectory of the package within the ``go_path``, including                     |
|   the ``src/`` prefix. May different from ``importpath`` due to vendoring.                       |
| * ``srcs``: list of source ``File``s.                                                            |
| * ``data``: list of data ``File``s.                                                              |
+--------------------------------+-----------------------------------------------------------------+

GoSDK
~~~~~

``GoSDK`` contains information about the Go SDK used in the toolchain.

+--------------------------------+-----------------------------------------------------------------+
| **Name**                       | **Type**                                                        |
+--------------------------------+-----------------------------------------------------------------+
| :param:`goos`                  | :type:`string`                                                  |
+--------------------------------+-----------------------------------------------------------------+
| The host operating system the SDK was built for.                                                 |
+--------------------------------+-----------------------------------------------------------------+
| :param:`goarch`                | :type:`string`                                                  |
+--------------------------------+-----------------------------------------------------------------+
| The host architecture the SDK was built for.                                                     |
+--------------------------------+-----------------------------------------------------------------+
| :param:`root_file`             | :type:`File`                                                    |
+--------------------------------+-----------------------------------------------------------------+
| A file in the SDK root directory. Used to determine ``GOROOT``.                                  |
+--------------------------------+-----------------------------------------------------------------+
| :param:`libs`                  | :type:`depset of File`                                          |
+--------------------------------+-----------------------------------------------------------------+
| Pre-compiled .a files for the standard library, built for the                                    |
| execution platform.                                                                              |
+--------------------------------+-----------------------------------------------------------------+
| :param:`headers`               | :type:`depset of File`                                          |
+--------------------------------+-----------------------------------------------------------------+
| .h files from pkg/include that may be included in assembly sources.                              |
+--------------------------------+-----------------------------------------------------------------+
| :param:`srcs`                  | :type:`depset of File`                                          |
+--------------------------------+-----------------------------------------------------------------+
| Source files for importable packages in the standard library.                                    |
| Internal, vendored, and tool packages might not be included.                                     |
+--------------------------------+-----------------------------------------------------------------+
| :param:`package_list`          | :type:`File`                                                    |
+--------------------------------+-----------------------------------------------------------------+
| A file containing a list of importable packages in the standard library.                         |
+--------------------------------+-----------------------------------------------------------------+
| :param:`tools`                 | :type:`depset of File`                                          |
+--------------------------------+-----------------------------------------------------------------+
| Executable files from pkg/tool built for the execution platform.                                 |
+--------------------------------+-----------------------------------------------------------------+
| :param:`go`                    | :type:`File`                                                    |
+--------------------------------+-----------------------------------------------------------------+
| The go binary file.                                                                              |
+--------------------------------+-----------------------------------------------------------------+

GoStdLib
~~~~~~~~

``GoStdLib`` contains information about the standard library being used for
compiling and linking. The standard library may be the pre-compiled library
from GoSDK_, or it may be another library compiled for the target mode.

+--------------------------------+-----------------------------------------------------------------+
| **Name**                       | **Type**                                                        |
+--------------------------------+-----------------------------------------------------------------+
| :param:`root_file`             | :type:`File`                                                    |
+--------------------------------+-----------------------------------------------------------------+
| A file or directory in the standard library root directory. Used to determine ``GOROOT``.        |
+--------------------------------+-----------------------------------------------------------------+
| :param:`libs`                  | :type:`list of File`                                            |
+--------------------------------+-----------------------------------------------------------------+
| .a files for the standard library, built for the target platform.                                |
+--------------------------------+-----------------------------------------------------------------+
| :param:`cache_dir`             | :type:`list of File`                                            |
+--------------------------------+-----------------------------------------------------------------+
| GOCACHE directory for the stdlib after running `go list`.                                        |
+--------------------------------+-----------------------------------------------------------------+
