Nogo Bzlmod configuration
==================

.. _nogo: /go/nogo.rst
.. _Bzlmod: /docs/go/core/bzlmod.md

Tests that verify nogo_ works when configured from ``MODULE.bazel``.

.. contents::

includes_excludes_test
-----------

Verifies that `go_library`_ targets can be built in default configurations with
nogo with includes and excludes being honored.
