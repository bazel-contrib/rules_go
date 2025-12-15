Nogo excludes-includes configuration
==================

.. _nogo: /go/nogo.rst

Tests that verify nogo_ `includes` and `excludes` works when configured from ``WORKSPACE.bazel``.

.. contents::

includes_excludes_test
-----------

Verifies that `go_library`_ targets can be built in default configurations with
nogo with includes and excludes being honored.
