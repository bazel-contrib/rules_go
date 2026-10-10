Basic go_path functionality
===========================

.. _go_path: /docs/go/core/rules.md#_go_path

Tests to ensure the basic features of `go_path`_ are working as expected.

go_path_test
------------

Consumes `go_path`_ rules built for the same set of packages in archive, copy,
and link modes and verifies that expected files are present in each mode.

embedsrcs_test
--------------

Builds a `go_path`_ for a package whose embedded file sits in a subdirectory
and verifies the file keeps its path relative to the package. The package and
directory names use only characters that appear in Bazel output directory
paths, which catches the output directory being removed character by character
instead of as a prefix.
