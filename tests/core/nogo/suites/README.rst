nogo analyzer suites
====================

.. _nogo: /go/nogo.rst

Tests the ``suites`` attribute of `nogo`_.

.. contents::

suites_test
-----------
Verifies that nogo runs every analyzer in a library that exports a slice of
them, and that an analyzer in both ``deps`` and a suite runs once: nogo rejects
duplicate analyzers, so a suite that repeats one from ``deps`` must not fail the
build.
