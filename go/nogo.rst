|logo| nogo build-time code analysis
====================================

.. _nogo: nogo.rst#nogo
.. _configuring-analyzers: nogo.rst#configuring-analyzers
.. _validation action: https://bazel.build/extending/rules#validation_actions
.. _Bzlmod: /docs/go/core/bzlmod.md#configuring-nogo
.. _go_library: /docs/go/core/rules.md#go_library
.. _analysis: https://godoc.org/golang.org/x/tools/go/analysis
.. _Analyzer: https://godoc.org/golang.org/x/tools/go/analysis#Analyzer
.. _GoInfo: providers.rst#GoInfo
.. _GoArchive: providers.rst#GoArchive
.. _vet: https://golang.org/cmd/vet/
.. _golangci-lint: https://github.com/golangci/golangci-lint
.. _staticcheck: https://staticcheck.io/
.. _sluongng/nogo-analyzer: https://github.com/sluongng/nogo-analyzer

.. role:: param(kbd)
.. role:: type(emphasis)
.. role:: value(code)
.. |mandatory| replace:: **mandatory value**
.. |logo| image:: nogo_logo.png
.. footer:: The ``nogo`` logo was derived from the Go gopher, which was designed by Renee French. (http://reneefrench.blogspot.com/) The design is licensed under the Creative Commons 3.0 Attributions license. Read this article for more details: http://blog.golang.org/gopher


``nogo`` is a tool that analyzes the source code of Go programs. It runs
in an action after the Go compiler in the Bazel Go rules and rejects sources that
contain disallowed coding patterns from the configured analyzers.

``nogo`` is a powerful tool for preventing bugs and code anti-patterns early
in the development process. It may be used to run the same analyses as `vet`_,
and you can write new analyses for your own code base.

.. contents:: .
  :depth: 2

-----

Setup
-----

Create a `nogo`_ target in a ``BUILD`` file in your workspace. The ``deps``
attribute of this target must contain labels all the analyzers targets that you
want to run.

.. code:: bzl

    load("@io_bazel_rules_go//go:def.bzl", "nogo")

    nogo(
        name = "my_nogo",
        deps = [
            # analyzer from the local repository
            ":importunsafe",
            # analyzer from a remote repository
            "@org_golang_x_tools//go/analysis/passes/printf:go_default_library",
        ],
        visibility = ["//visibility:public"], # must have public visibility
    )

    go_library(
        name = "importunsafe",
        srcs = ["importunsafe.go"],
        importpath = "importunsafe",
        deps = ["@org_golang_x_tools//go/analysis:go_default_library"],
        visibility = ["//visibility:public"],
    )

Pass a label for your `nogo`_ target to ``go_register_nogo`` in your
``WORKSPACE`` file. When using ``MODULE.bazel``, see the Bzlmod_ documentation
instead.

.. code:: bzl

    load("@io_bazel_rules_go//go:deps.bzl", "go_rules_dependencies", "go_register_nogo")
    go_rules_dependencies()
    go_register_toolchains(version = "1.23.1")
    go_register_nogo(
      nogo = "@//:my_nogo"  # my_nogo is in the top-level BUILD file of this workspace
      includes = ["@//:__subpackages__"],  # Labels to lint. By default only lints code in workspace.
      excludes = ["@//generated:__subpackages__"],  # Labels to exclude.
    )

**NOTE**: You must include ``"@//"`` prefix when referring to targets in the local
workspace. Also note that you cannot use this to refer to bzlmod repos, as the labels
don't go though repo mapping.

The `nogo`_ rule will generate a program that executes all the supplied
analyzers at build-time. The generated ``nogo`` program will run alongside the
compiler when building any Go target (e.g. `go_library`_) within your workspace,
even if the target is imported from an external repository. However, ``nogo``
will not run when targets from the current repository are imported into other
workspaces and built there.

To run all the ``golang.org/x/tools`` analyzers, use ``@io_bazel_rules_go//:tools_nogo``.

.. code:: bzl

    load("@io_bazel_rules_go//go:deps.bzl", "go_rules_dependencies", "go_register_toolchains")
    go_rules_dependencies()
    go_register_toolchains(nogo = "@io_bazel_rules_go//:tools_nogo")

To run the analyzers from ``tools_nogo`` together with your own analyzers, use
the ``TOOLS_NOGO`` list of dependencies.

.. code:: bzl

    load("@io_bazel_rules_go//go:def.bzl", "nogo", "TOOLS_NOGO")

    nogo(
        name = "my_nogo",
        deps = TOOLS_NOGO + [
            # analyzer from the local repository
            ":importunsafe",
        ],
        visibility = ["//visibility:public"], # must have public visibility
    )

    go_library(
        name = "importunsafe",
        srcs = ["importunsafe.go"],
        importpath = "importunsafe",
        deps = ["@org_golang_x_tools//go/analysis:go_library"],
        visibility = ["//visibility:public"],
    )

Usage
---------------------------------

``nogo``, upon configured, will be invoked automatically when building any Go target in your
workspace.  If any of the analyzers reject the program, the build will fail. However, since
``nogo`` runs in a `validation action`_ that is separate from compilation, you can use
``--keep_going`` to have compilation continue and see all ``nogo`` findings, not just those
from the first failing target. You can also specify ``--norun_validations`` to disable all
validations, including ``nogo``.

Note: Since the action that runs ``nogo`` doesn't fail if ``nogo`` produces findings, it
is not possible to debug it with ``--sandbox_debug``. If necessary, set the ``debug``
attribute of the ``nogo`` rule to ``True`` to have ``nogo`` fail in this case.

``nogo`` will run on all Go targets in your workspace, including tests and binary targets.
When using WORKSPACE, it will also run on targets that are imported from other workspaces
by default. You could exclude the external repositories from ``nogo`` by using the
`exclude_files` regex in `configuring-analyzers`_. With Bzlmod, external repositories are
not validated with ``nogo`` by default. See the Bzlmod_ guide for more information
on how to configure the ``nogo`` scope in this case.

Relationship with other linters
~~~~~~~~~~~~~~~~~~~~~

In Golang, a linter is composed of multiple parts:

- A collection of rules (checks) that define different validations against the source code

- Optionally, each rules could be coupled with a fixer that can automatically fix the code.

- A configuration framework that allows users to enable/disable rules, and configure the rules.

- A runner binary that orchestrate the above components.

To help with the above, Go provides a framework called `analysis`_ that allows
you to write a linter in a modular way. In which, you could define each rules as a separate
`Analyzer`_, and then compose them together in a runner binary.

For example, `golangci-lint`_ or `staticcheck`_ are popular linters that are composed of multiple
analyzers, each of which is a collection of rules.

``nogo`` is a runner binary that runs a collection of analyzers while leveraging Bazel's
action orchestration framework. In particular, ``nogo`` is run as part of rules_go GoCompilePkg
action, and it is run in parallel with the Go compiler. This allows ``nogo`` to benefit from
Bazel's incremental build and caching as well as the Remote Build Execution framework.

There are examples of how to re-use the analyzers from `golangci-lint`_ and `staticcheck`_ in
`nogo`_ here: `sluongng/nogo-analyzer`_.

Should I use ``nogo`` or ``golangci-lint``?
~~~~~~~~~~~~~~~~~~~~~

Because ``nogo`` benefits from Bazel's incremental build and caching, it is more suitable for
large code bases. If you have a smaller code base, you could use ``golangci-lint`` instead.

If ``golangci-lint`` takes a really long time to run in your repository, you could try to use
``nogo`` instead.

As of the moment of this writing, there is no way for ``nogo`` to apply the fixers coupled
with the analyzers. So separate linters such as ``golangci-lint`` or ``staticcheck`` are more
ergonomic to apply the fixes to the code base.

Writing and registering analyzers
---------------------------------

``nogo`` analyzers are Go packages that declare a variable named ``Analyzer``
of type `Analyzer`_ from package `analysis`_. Each analyzer is invoked once per
Go package, and is provided the abstract syntax trees (ASTs) and type
information for that package, as well as relevant results of analyzers that have
already been run. For example:

.. code:: go

    // package importunsafe checks whether a Go package imports package unsafe.
    package importunsafe

    import (
      "strconv"

      "golang.org/x/tools/go/analysis"
    )

    var Analyzer = &analysis.Analyzer{
      Name: "importunsafe",
      Doc: "reports imports of package unsafe",
      Run: run,
    }

    func run(pass *analysis.Pass) (interface{}, error) {
      for _, f := range pass.Files {
        for _, imp := range f.Imports {
          path, err := strconv.Unquote(imp.Path.Value)
          if err == nil && path == "unsafe" {
            pass.Reportf(imp.Pos(), "package unsafe must not be imported")
          }
        }
      }
      return nil, nil
    }

Any diagnostics reported by the analyzer will stop the build. Do not emit
diagnostics unless they are severe enough to warrant stopping the build.

Pass labels for these targets to the ``deps`` attribute of your `nogo`_ target,
as described in the `Setup`_ section.

Configuring analyzers
~~~~~~~~~~~~~~~~~~~~~

By default, ``nogo`` analyzers will emit diagnostics for all Go source files
built by Bazel. This behavior can be changed with a JSON configuration file.

The top-level JSON object in the file must be keyed by the name of the analyzer
being configured. These names must match the ``Analyzer.Name`` of the registered
analysis package. The JSON object's values are themselves objects which may
contain the following key-value pairs:

+----------------------------+---------------------------------------------------------------------+
| **Key**                    | **Type**                                                            |
+----------------------------+---------------------------------------------------------------------+
| ``"description"``          | :type:`string`                                                      |
+----------------------------+---------------------------------------------------------------------+
| Description of this analyzer configuration.                                                      |
+----------------------------+---------------------------------------------------------------------+
| ``"only_files"``           | :type:`dictionary, string to string`                                |
+----------------------------+---------------------------------------------------------------------+
| Specifies files that this analyzer will emit diagnostics for.                                    |
| Its keys are regular expression strings matching Go file names, and its values are strings       |
| containing a description of the entry.                                                           |
| If both ``only_files`` and ``exclude_files`` are empty, this analyzer will emit diagnostics for  |
| all Go files built by Bazel.                                                                     |
+----------------------------+---------------------------------------------------------------------+
| ``"exclude_files"``        | :type:`dictionary, string to string`                                |
+----------------------------+---------------------------------------------------------------------+
| Specifies files that this analyzer will not emit diagnostics for.                                |
| Its keys and values are strings that have the same semantics as those in ``only_files``.         |
| Keys in ``exclude_files`` override keys in ``only_files``. If a .go file matches a key present   |
| in both ``only_files`` and ``exclude_files``, the analyzer will not emit diagnostics for that    |
| file.                                                                                            |
+----------------------------+---------------------------------------------------------------------+
| ``"analyzer_flags"``       | :type:`dictionary, string to string`                                |
+----------------------------+---------------------------------------------------------------------+
| Passes on a set of flags as defined by the Go ``flag`` package to the analyzer via the           |
| ``analysis.Analyzer.Flags`` field. Its keys are the flag names *without* a ``-`` prefix, and its |
| values are the flag values. nogo will exit with an error upon receiving flags not recognized by  |
| the analyzer or upon receiving ill-formatted flag values as defined by the corresponding         |
| ``flag.Value`` specified by the analyzer.                                                        |
+----------------------------+---------------------------------------------------------------------+

``nogo`` also supports a special key to specify the same config for all analyzers, even if they are
not explicitly specified called ``_base``. See below for an example of its usage.

Example
^^^^^^^

The following configuration file configures the analyzers named ``importunsafe``
and ``unsafedom``. Since the ``loopclosure`` analyzer is not explicitly
configured, it will emit diagnostics for all Go files built by Bazel.
``unsafedom`` will receive a flag equivalent to ``-block-unescaped-html=false``
on a command line driver.

.. code:: json

    {
      "_base": {
        "description": "Base config that all subsequent analyzers, even unspecified will inherit.",
        "exclude_files": {
          "third_party/": "exclude all third_party code for all analyzers"
        }
      },
      "importunsafe": {
        "exclude_files": {
          "src/foo\\.go": "manually verified that behavior is working-as-intended",
          "src/bar\\.go": "see issue #1337"
        }
      },
      "unsafedom": {
        "only_files": {
          "src/js/.*": ""
        },
        "exclude_files": {
          "src/(third_party|vendor)/.*": "enforce DOM safety requirements only on first-party code"
        },
        "analyzer_flags": {
            "block-unescaped-html": "false",
        },
      }
    }

This label referencing this configuration file must be provided as the
``config`` attribute value of the ``nogo`` rule.

.. code:: bzl

    nogo(
        name = "my_nogo",
        deps = [
            ":importunsafe",
            ":unsafedom",
            "@analyzers//:loopclosure",
        ],
        config = "config.json",
        visibility = ["//visibility:public"],
    )

Running vet
-----------

`vet`_ is a tool that examines Go source code and reports correctness issues not
caught by Go compilers. It is included in the official Go distribution. Vet
runs analyses built with the Go `analysis`_ framework. nogo uses the
same framework, which means vet checks can be run with nogo.

You can choose to run a safe subset of vet checks alongside the Go compiler by
setting ``vet = True`` in your `nogo`_ target. This will only run vet checks
that are believed to be 100% accurate (the same set run by ``go test`` by
default).

.. code:: bzl

    nogo(
        name = "my_nogo",
        vet = True,
        visibility = ["//visibility:public"],
    )

Setting ``vet = True`` is equivalent to adding the ``atomic``, ``bools``,
``buildtag``, ``nilfunc``, and ``printf`` analyzers from
``@org_golang_x_tools//go/analysis/passes`` to the ``deps`` list of your
``nogo`` rule.


See the full list of available nogo checks:

.. code:: shell

    bazel query 'kind(go_library, @org_golang_x_tools//go/analysis/passes/...)'


API
---

nogo
~~~~

This generates a program that analyzes the source code of Go programs. It
runs alongside the Go compiler in the Bazel Go rules and rejects programs that
contain disallowed coding patterns.

Attributes
^^^^^^^^^^

+----------------------------+-----------------------------+---------------------------------------+
| **Name**                   | **Type**                    | **Default value**                     |
+----------------------------+-----------------------------+---------------------------------------+
| :param:`name`              | :type:`string`              | |mandatory|                           |
+----------------------------+-----------------------------+---------------------------------------+
| A unique name for this rule.                                                                     |
+----------------------------+-----------------------------+---------------------------------------+
| :param:`deps`              | :type:`label_list`          | :value:`None`                         |
+----------------------------+-----------------------------+---------------------------------------+
| List of Go libraries that will be linked to the generated nogo binary.                           |
|                                                                                                  |
| These libraries must declare an ``analysis.Analyzer`` variable named `Analyzer` to ensure that   |
| the analyzers they implement are called by nogo.                                                 |
|                                                                                                  |
+----------------------------+-----------------------------+---------------------------------------+
| :param:`config`            | :type:`label`               | :value:`None`                         |
+----------------------------+-----------------------------+---------------------------------------+
| JSON configuration file that configures one or more of the analyzers in ``deps``.                |
+----------------------------+-----------------------------+---------------------------------------+
| :param:`vet`               | :type:`bool`                | :value:`False`                        |
+----------------------------+-----------------------------+---------------------------------------+
| If true, a safe subset of vet checks will be run by nogo (the same subset run                    |
| by ``go test ``).                                                                                |
+----------------------------+-----------------------------+---------------------------------------+

Example
^^^^^^^

.. code:: bzl

    nogo(
        name = "my_nogo",
        deps = [
            ":importunsafe",
            ":otheranalyzer",
            "@analyzers//:unsafedom",
        ],
        config = ":config.json",
        vet = True,
        visibility = ["//visibility:public"],
    )
