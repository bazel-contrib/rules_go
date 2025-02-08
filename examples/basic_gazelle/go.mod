// Go uses this file to track module dependencies and to mark the root directory
// of a Go module. This file is maintained with Go tools outside of Bazel.
// Gazelle's go_deps module extension imports dependency declarations
// from this file.

module github.com/bazel-contrib/rules_go/examples/basic_gazelle

go 1.23.4

require golang.org/x/net v0.34.0
