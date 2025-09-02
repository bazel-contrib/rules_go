.. _#1486 : https://github.com/bazel-contrib/rules_go/issues/1486

Cgo static initialization with `alwayslink = True`
===================================================

test_side_effect_go
-------------------

This test verifies that C++ static initializers in `cc_library` targets with `alwayslink = True` are
executed when linked into a `go_test` or `go_binary` with `cgo = True`. This is a regression test
for issue `#1486`_. The test also includes a `side_effect_wrapper` library to ensure that the side
effect is executed exactly once, even if imported multiple times through different targets.

test_side_effect_cc
-------------------
This is a C++ test included as a reference. It has the same dependencies as `test_side_effect_go`
and confirms the expected behavior in a pure C++ environment, where the static initializer is always
executed
