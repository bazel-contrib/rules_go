Core Go rules
=============

.. _"Make variable": https://docs.bazel.build/versions/master/be/make-variables.html
.. _Bourne shell tokenization: https://docs.bazel.build/versions/master/be/common-definitions.html#sh-tokenization
.. _Gazelle: https://github.com/bazelbuild/bazel-gazelle
.. _GoArchive: providers.rst#GoArchive
.. _GoPath: providers.rst#GoPath
.. _GoInfo: providers.rst#GoInfo
.. _build constraints: https://golang.org/pkg/go/build/#hdr-Build_Constraints
.. _cc_library deps: https://docs.bazel.build/versions/master/be/c-cpp.html#cc_library.deps
.. _cgo: http://golang.org/cmd/cgo/
.. _config_setting: https://docs.bazel.build/versions/master/be/general.html#config_setting
.. _data dependencies: https://bazel.build/concepts/dependencies#data-dependencies
.. _goarch: modes.rst#goarch
.. _goos: modes.rst#goos
.. _mode attributes: modes.rst#mode-attributes
.. _nogo: nogo.rst#nogo
.. _pure: modes.rst#pure
.. _race: modes.rst#race
.. _msan: modes.rst#msan
.. _select: https://docs.bazel.build/versions/master/be/functions.html#select
.. _shard_count: https://docs.bazel.build/versions/master/be/common-definitions.html#test.shard_count
.. _static: modes.rst#static
.. _test_arg: https://docs.bazel.build/versions/master/user-manual.html#flag--test_arg
.. _test_filter: https://docs.bazel.build/versions/master/user-manual.html#flag--test_filter
.. _test_env: https://docs.bazel.build/versions/master/user-manual.html#flag--test_env
.. _test_runner_fail_fast: https://docs.bazel.build/versions/master/command-line-reference.html#flag--test_runner_fail_fast
.. _define and register a C/C++ toolchain and platforms: https://bazel.build/extending/toolchains#toolchain-definitions
.. _bazel: https://pkg.go.dev/github.com/bazelbuild/rules_go/go/tools/bazel?tab=doc
.. _introduction: /docs/go/core/rules.md#introduction
.. _rules: /docs/go/core/rules.md#rules
.. _examples: /docs/go/core/examples.md
.. _defines-and-stamping: /docs/go/core/defines_and_stamping.md#defines-and-stamping
.. _stamping-with-the-workspace-status-script: /docs/go/core/defines_and_stamping.md#stamping-with-the-workspace-status-script
.. _embedding: /docs/go/core/embedding.md#embedding
.. _cross-compilation: /docs/go/core/cross_compilation.md#cross-compilation
.. _platform-specific-dependencies: /docs/go/core/platform-specific_dependencies.md#platform-specific-dependencies



.. role:: param(kbd)
.. role:: type(emphasis)
.. role:: value(code)
.. |mandatory| replace:: **mandatory value**

These are the core go rules, required for basic operation.
The intent is that these rules are sufficient to match the capabilities of the normal go tools.

.. contents:: :depth: 2

-----

Introduction
------------

This section has been moved to introduction_.


Rules
-----

This section has been moved to rules_.

The examples pertaining to each rule have been moved to examples_.


Defines and stamping
--------------------

This section has been moved to defines-and-stamping_.


Stamping with the workspace status script
~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~

This section has been moved to stamping-with-the-workspace-status-script_.


Embedding
---------

This section has been moved to embedding_.


Cross compilation
-----------------

This section has been moved to cross-compilation_.

Platform-specific dependencies
~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~

This section has been moved to platform-specific-dependencies_.
