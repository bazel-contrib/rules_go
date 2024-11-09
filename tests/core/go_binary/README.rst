Basic go_binary functionality
=============================

.. _go_binary: /docs/go/core/rules.md#_go_binary
.. _#2168: https://github.com/bazelbuild/rules_go/issues/2168
.. _#2463: https://github.com/bazelbuild/rules_go/issues/2463

Tests to ensure the basic features of go_binary are working as expected.

hello
-----

Hello is a basic "hello world" program that doesn't do anything interesting.
Useful as a primitive smoke test -- if this doesn't build, nothing will.

out_test
--------

Tests that a `go_binary`_ rule can write its executable file with a custom name
in the package directory (not the mode directory).

package_conflict_test
---------------------

Tests that linking multiple packages with the same path (`importmap`) is an
error.

goos_pure_bin
-------------

Tests that specifying the `goos` attribute on a `go_binary`_ target to be
different than the host os forces the pure mode to be on. This is achieved
by including a broken cgo file in the sources for the build.

many_deps
---------

Test that a `go_binary`_ with many imports with long names can be linked. This
makes sure we don't exceed command-line length limits with -I and -L flags.
Verifies #1637.

stamp_test
----------
Test that the `go_binary`_ ``x_defs`` attribute works correctly, both in a
binary and in an embedded library. Tests regular stamps and stamps that
depend on values from the workspace status script. Verifies #2000.

pie_test
--------
Tests that specifying the ``linkmode`` attribute on a `go_binary`_ target to be
pie produces a position-independent executable and that no specifying it produces
a position-dependent binary.

static_test
-----------
Test that `go_binary`_ rules with ``static = "on"`` with and without cgo
produce static binaries. Verifies `#2168`_.

This test only runs on Linux. The darwin external linker cannot produce
static binaries since there is no static version of C runtime libraries.

tags_bin
--------
Checks that setting ``gotags`` affects source filtering. This binary won't build
without a specific tag being set.

prefix
------
This binary has a name that conflicts with a subdirectory. Its output file
name should not have this conflict. Verifies `#2463`_.

godebug_test
------------
Test that the ``godebug_default`` sets the default ``GODEBUG`` variables.
