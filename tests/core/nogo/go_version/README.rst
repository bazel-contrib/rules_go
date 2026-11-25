nogo Go version plumbing
========================

Tests that `nogo` receives the selected SDK Go version during type checking.

go_version_test
---------------
Builds a package with nogo enabled and verifies two things:

* The ``RunNogo`` action includes ``-go_version`` set to the active SDK version.
  The flag value is the raw ``go.sdk.version`` string (for example
  ``1.24.3``), without a leading ``go`` prefix.
* On SDKs that expose the relevant ``go/types`` APIs, ``nogo_main`` normalizes
  that flag to the ``go/types`` format (for example ``go1.24.3``), and the
  analyzer sees that normalized version through ``pass.Pkg.GoVersion()`` and
  ``pass.TypesInfo.FileVersions``.

The test intentionally does not cover ``gc_goopts`` ``-lang`` overrides; nogo
still ignores those today.
