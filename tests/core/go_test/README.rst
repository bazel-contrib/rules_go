Basic go_test functionality
===========================

.. _go_test: /docs/go/core/rules.md#_go_test
.. _#1877: https://github.com/bazelbuild/rules_go/issues/1877
.. _#34129: https:////github.com/golang/go/issues/34129
.. _#2749: https://github.com/bazelbuild/rules_go/issues/2749

Tests to ensure that basic features of `go_test`_ are working as expected.

.. contents::

internal_test
-------------

Test that a `go_test`_ rule that adds white box tests to an embedded package works.
This builds a library with `lib.go <lib.go>`_ and then a package with an
`internal test <internal_test.go>`_ that contains the test case.
It uses x_def stamped values to verify the library names are correct.

external_test
-------------

Test that a `go_test`_ rule that adds black box tests for a dependant package works.
This builds a library with `lib.go <lib.go>`_ and then a package with an
`external test <external_test.go>`_ that contains the test case.
It uses x_def stamped values to verify the library names are correct.

combined_test
-------------
Test that a `go_test`_ rule that adds both white and black box tests for a
package works.
This builds a library with `lib.go <lib.go>`_ and then a one merged with the
`internal test <internal_test.go>`_, and then another one that depends on it
with the `external test <external_test.go>`_.
It uses x_def stamped values to verify that all library names are correct.
Verifies #413

flag_test
---------
Test that a `go_test`_ rule that adds flags, even in the main package, can read
the flag.
This does not even build a library, it's a test in the main package with no
dependancies that checks it can declare and then read a flag.
Verifies #838

only_testmain_test
------------------
Test that an `go_test`_ that contains a ``TestMain`` function but no tests
still builds and passes.

external_importmap_test
----------------------

Test that an external test package in `go_test`_ is compiled with the correct
``importmap`` for the library under test. This is verified by defining an
interface in the library under test and implementing it in a separate
dependency.

Verifies #1538.

pwd_test
--------

Checks that the ``PWD`` environment variable is set to the current directory
in the generated test main before running a test. This matches functionality
in ``go test``.

Verifies #1561.

data_test
---------

Checks that data dependencies, including those inherited from ``deps`` and
``embed``, are visible to tests at run-time. Source files should not be
visible at run-time.

test_fail_fast_test
----------------

Checks that ``--test_runner_fail_fast`` actually enables stopping test execution after
the first failure.

Verifies #3055.

test_filter_test
----------------

Checks that ``--test_filter`` actually filters out test cases.

testmain_import_test
----------------

Check if all packages in all source files are imported to test main, to ensure
a consistent test behaviour. This ensures a consistent behaviour when thinking
about global indirect depencencies.

tags_test
---------

Checks that setting ``gotags`` affects source filtering. The test will fail
unless a specific tag is set.

indirect_import_test
--------------------

Checks that an external test can import another package that imports the library
under test. The other package should be compiled against the internal test
package, not the library under test. Verifies `#1877`_.

testmain_without_exit
---------------------

Checks that TestMain without calling os.Exit directly works.
Verifies `#34129`_ from Go 1.15.

wrapper_test
------------

Checks that a ``go_test`` can be executed by another test in a subdirectory.
Verifies `#2749`_.

testmain_pwd_test
-----------------

Checks that a test which spawns a subprocess of the test binary
is run inside the correct directory.

fuzz_test
---------

Checks that a ``go_test`` with a fuzz target builds correctly.
