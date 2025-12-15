.. _#1486 : https://github.com/bazel-contrib/rules_go/issues/1486

Cgo and dynamic initialization with `alwayslink = True`
========================================================

test_side_effect_go
-------------------
This test verifies that the dynamic initialization of C++ variables with static storage duration in
`cc_library` targets with `alwayslink = True` is performed when linked into a `go_test` or
`go_binary` with `cgo = True`. This is a regression test for issue `#1486`_. The test also
includes a `lib_wrapper` library to ensure that there are no linker errors and that the side effect
is performed exactly once, even if it is imported multiple times through different targets.

test_side_effect_cc
-------------------
This is a C++ test included as a reference. It has the same dependencies as `test_side_effect_go`
and confirms the expected behavior in a pure C++ environment, where the dynamic initialization of
variables with static storage duration is always performed.
