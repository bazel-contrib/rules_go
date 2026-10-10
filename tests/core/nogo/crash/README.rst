nogo crashes
============

.. _nogo: /go/nogo.rst

Tests that a crash of the `nogo`_ binary fails the build with its output.

.. contents::

crash_test
----------
Verifies that a nogo binary that panics before running any analyzer (here, in
an analyzer's ``init``) fails the build with the panic message. The panic exits
with the same code as a run with findings, and was previously reported only as
a missing ``.facts`` output.
