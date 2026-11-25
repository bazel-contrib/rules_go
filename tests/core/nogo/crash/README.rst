nogo crashes
============

.. _nogo: /go/nogo.rst

Tests that a crash of the `nogo`_ binary fails the build with its output.

.. contents::

crash_test
----------
Verifies that a nogo binary that panics before running any analyzer (here, in
an analyzer's ``init``) fails the build with the panic message. The panic exits
with status 2, which nogo used to also return for a run with findings, so it was
reported only as a missing ``.facts`` output.
