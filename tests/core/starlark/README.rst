Starlark unit tests
=======================

common_test_suite
---------

Checks that ``has_shared_lib_extension`` from ``//go/private:common.bzl``
correctly matches shared library filenames, which may optionally have a version
number at the end.

extensions_test_suite
---------------------

Checks that ``drop_shadowed_dependency_toolchains`` from
``//go/private:extensions.bzl`` only drops toolchains of non-root SDKs whose
exec platform a root module SDK covers, treating an empty ``goos`` and
``goarch`` as the host platform.
