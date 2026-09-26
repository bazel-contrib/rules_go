nogo runtime
============

This package loads Go packages, imports and exports types and analysis facts,
executes analyzers, filters diagnostics, and writes suggested fixes. ``driver.go``
provides the entry point, ``analyze.go`` runs the analyzer graph, and
``imports.go`` owns package loading and the private type/fact artifact format.
The unit tests cover suggested-fix merging, nolint directives, and Go-version
normalization.

The generated nogo launcher supplies analyzers and configuration to ``Run`` and
owns parameter-file expansion, logging, and process exit. The bootstrap builder
in the parent directory prepares each analysis action and executes the launcher;
it does not import this package. Keep their small process protocol (exit codes
and patch filename) synchronized.

The runtime is a ``go_tool_library`` built under nogo's tool transition to avoid
bootstrap recursion. It uses x/tools' internal facts package through rules_go's
Bazel dependency setup.
