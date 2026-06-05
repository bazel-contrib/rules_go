# BuildInfo Metadata Tests

This directory contains tests for the buildInfo metadata changes that implement Go 1.18+ runtime/debug.BuildInfo embedding with dependency version information.

## Test Structure

### Runtime Integration Tests

1. **metadata_test.go** - Tests internal dependency version collection
   - Builds a binary with a chain of internal dependencies (leaf_lib -> mid_lib -> top_lib)
   - Validates that `runtime/debug.ReadBuildInfo()` returns build information
   - Checks that all transitive Go dependencies are listed in the BuildInfo
   - Verifies the main package path is set correctly

2. **external_deps_test.go** - Tests external dependency version collection
   - Builds a binary that depends on golang.org/x/sys
   - Validates that external dependencies appear in BuildInfo
   - Checks that real versions (not "(devel)") are used for external dependencies with package_metadata

## Implementation Changes

The tests cover the refactored buildInfo implementation that addresses review comments:

### Key Changes from Committed Version (ea1600b9)

1. **buildinfo_aspect.bzl** - Changed provider schema:
   - OLD: Single `version_map` depset of (importpath, version) tuples
   - NEW: Separate `importpaths` and `metadata_providers` depsets
   - Avoids quadratic memory usage by deferring version materialization to link time
   - Only traverses `deps` and `embed` attributes instead of all attributes

2. **archive.bzl** - Removed quadratic aggregation:
   - OLD: Collected `_buildinfo_deps` tuples in each archive
   - NEW: No buildinfo aggregation in archives
   - BuildInfo metadata now collected only via aspect

3. **link.bzl** - Deferred materialization:
   - OLD: Received pre-computed version tuples from archives
   - NEW: Receives BuildInfoMetadata provider and materializes depsets at link time
   - Extracts versions from package_metadata providers on-demand

4. **binary.bzl** - Simplified metadata passing:
   - OLD: Created version_map file by collecting and deduplicating tuples
   - NEW: Passes BuildInfoMetadata provider directly to link action

## Test Files

```
tests/core/buildinfo/
├── BUILD.bazel                          # Test target definitions
├── README.md                             # This file
├── leaf_lib.go                          # Bottom of dependency chain
├── mid_lib.go                           # Middle of dependency chain
├── top_lib.go                           # Top of dependency chain
├── metadata_bin.go                      # Binary that prints BuildInfo
├── metadata_test.go                     # Runtime test for internal deps
├── external_deps_bin.go                 # Binary with external deps
└── external_deps_test.go                # Runtime test for external deps
```

## Running the Tests

```bash
# Run all buildinfo tests
bazel test //tests/core/buildinfo:buildinfo

# Run individual tests
bazel test //tests/core/buildinfo:metadata_test
bazel test //tests/core/buildinfo:external_deps_test
```

## Expected Behavior

### Internal Dependencies (metadata_test)

The test validates that binaries built with rules_go embed proper BuildInfo including:
- Main package path
- Go version
- All transitive Go dependencies with their import paths
- For internal monorepo packages: version shows as "(devel)"

### External Dependencies (external_deps_test)

The test validates that external dependencies from go_repository:
- Are included in the dependency list
- Have real version strings (e.g., "v0.1.0") instead of "(devel)"
- Version information comes from package_metadata set in REPO.bazel

## Notes

- The BuildInfoMetadata provider is internal to the binary rule implementation and not exposed on targets
- The aspect is automatically applied to go_binary targets via the deps and embed attributes
- Tests focus on runtime behavior validation via `runtime/debug.ReadBuildInfo()`
