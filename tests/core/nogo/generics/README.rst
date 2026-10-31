nogo analyzers run against code using generics
==============================================

.. _nogo: /go/nogo.rst
.. _buildssa: https://pkg.go.dev/golang.org/x/tools/go/analysis/passes/buildssa
.. _nilness: https://pkg.go.dev/golang.org/x/tools/go/analysis/passes/nilness

Tests to ensure that `nogo`_ analyzers that run on code using generics get correct
type instantiation information.

.. contents::

generics_test
-------------

Verifies that code using generic types gets loaded including all type instantiation
information, so that analyzers based on the `buildssa`_ analyzer (such as `nilness`_) get
a complete picture of all types in the code.

It also checks a leaf-only nogo analysis of a diamond import graph: two
intermediate packages expose the same generic type from an import-mapped
dependency through different import aliases. Its type argument is a standard-library
type also imported directly by the leaf. The intermediates
have no fact-producing analyzers, so their type exports must remain available.
