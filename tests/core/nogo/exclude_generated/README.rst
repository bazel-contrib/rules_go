nogo exclude_generated
======================

.. _nogo: /go/nogo.rst

Tests the ``exclude_generated`` key of a `nogo`_ config.

.. contents::

exclude_generated_test
----------------------
Verifies that with ``exclude_generated`` set, nogo emits no diagnostics for a
file marked ``// Code generated ... DO NOT EDIT.`` but still does for a
hand-written file, and that without it the generated file is checked like any
other.
