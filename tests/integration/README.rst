Integration tests
=================

This folder is intended to hold larger scale test that check that rules_go
works correctly in the real world, rather than in isolated single feature
tests.

If the unit tests were correct and exhaustive this directory should in theory
be redundant, but in practice it helps catch many issues and points to places
where more unit tests are needed.

Contents
--------

.. Child list start

* `Gazelle functionality <gazelle/README.rst>`_
* `Go 1.27 cgo linker flags <cgo_ldflags/README.rst>`_
* `Popular repository tests <popular_repos/README.rst>`_
* `Reproducibility <reproducibility/README.rst>`_
* `Functionality related to @go_googleapis <googleapis/README.rst>`_

.. Child list end

