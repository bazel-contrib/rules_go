!/bin/bash
# This script checks if a given binary contains an expected RPATH.

set -e

# Arguments are passed from the `sh_test` rule in BUILD.bazel.
BINARY_PATH=$1
EXPECTED_RPATH=$2

# Validate that the necessary arguments were provided.
if [[ -z "$BINARY_PATH" || -z "$EXPECTED_RPATH" ]]; then
  echo "Error: Missing arguments."
  echo "Usage: $0 <path_to_binary> <expected_rpath>"
  exit 1
fi


# Use `readelf` to look for RPATH or RUNPATH in the dynamic section of the binary.
RPATH_OUTPUT=$(readelf -d "$BINARY_PATH" | grep -E 'RPATH|RUNPATH' || true)

if echo "${RPATH_OUTPUT}" | grep -q "$EXPECTED_RPATH"; then
  echo "SUCCESS: Found expected RPATH on Linux."
  exit 0
else
  echo "FAILURE: Did not find expected RPATH on Linux."
  exit 1
fi
