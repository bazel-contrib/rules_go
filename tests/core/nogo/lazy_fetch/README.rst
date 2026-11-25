nogo is not fetched without Go targets
======================================

lazy_fetch_test
---------------
Builds a non-Go target that goes through toolchain resolution in a module
that depends on rules_go and registers its Go toolchains. Resolving any
toolchain type loads every registered toolchain, so the generated Go
toolchain declarations must not load the Go rules, which would fetch
``@io_bazel_rules_nogo``. The test checks that the repository was not fetched.
