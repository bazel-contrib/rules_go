nogo test with coverage
=======================

.. _nogo: /go/nogo.rst
.. _#3769: https://github.com/bazelbuild/rules_go/issues/3769
.. _#1940: https://github.com/bazelbuild/rules_go/issues/1940
.. _#2146: https://github.com/bazelbuild/rules_go/issues/2146

Tests to ensure that `nogo`_ works with coverage.

coverage_cgo_test
-------------
Checks that `nogo`_ works when both cgo and coverage are enabled. With coverage
instrumentation modifying source files, and cgo compilation changing files paths,
`nogo`_ should be running static analysis against original source files, instead
of modified ones. Verifies `#3769`_.

coverage_test
-------------
Checks that `nogo`_ works when coverage is enabled. All covered libraries gain
an implicit dependencies on ``//go/tools/coverdata``, which is a
`go_tool_library`_, which isn't built with `nogo`_. We should be able to
handle libraries like this that do not have serialized facts. Verifies `#1940`_.

Also checks that `nogo`_ itself can be built with coverage enabled.
Verifies `#2146`_.

gen_code_test
-------------
Checks how `nogo`_ should not run on source code that was generated as part of
rules_go's coverage implementation.
