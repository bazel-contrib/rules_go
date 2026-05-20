Nogo under legacy host mode
===========================

.. _go_library: /docs/go/core/rules.md#_go_library
.. _nogo: /docs/go/nogo.rst

Tests that `nogo`_ behaves correctly when a `go_library`_ is reached via
Bazel's legacy ``cfg = "host"`` configuration.

.. contents::

TestHostModeCleanLibBuilds
--------------------------
Verifies that a clean `go_library`_ reached through ``cfg = "host"`` builds
successfully. This catches two classes of regression at once:

* Infinite recursion through nogo's own dependency graph. ``nogo`` is itself
  a Go binary built from `go_library`_ analyzer deps, and ``cfg = "exec"`` does
  not reset ``//go/private:request_nogo``. Without a guard, analyzing any Go
  target would recursively force analysis of nogo, then of its analyzer libs,
  forever. The guard is the incoming ``go_tool_transition`` on the ``_nogo``
  rule (in ``go/private/rules/nogo.bzl``), which collapses the nogo alias to
  ``default_nogo`` for nogo's transitive deps. The test exercises the guard by
  configuring a real analyzer go_library — analysis must walk that whole
  chain. A passing test means the guard fires under ``cfg = "host"`` too.
* The nogo binary being built for the wrong execution platform, which would
  surface at runtime as ``exec format error``.

TestHostModeNogoActuallyRuns
----------------------------
Verifies that, when the host-mode wrapper propagates the ``_validation``
output group, nogo runs against the wrapped `go_library`_ and its findings
fail the build (the test wraps a library containing a function named ``Foo``
and asserts the build fails with the custom analyzer's diagnostic).
