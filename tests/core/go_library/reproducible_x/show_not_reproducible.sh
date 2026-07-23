#!/usr/bin/env bash
set -euo pipefail

cd "${BUILD_WORKSPACE_DIRECTORY}"
BAZEL_BIN="$(bazel info bazel-bin)"

tmp_output="$(mktemp -d)"

function build_and_show_hash() {
  echo "Building..."
  bazel build //tests/core/go_library/reproducible_x:calculator
  printf "\n\n\n"
  echo "Taking hash of .x file"
  cp "$BAZEL_BIN/tests/core/go_library/reproducible_x/calculator.x" "$tmp_output/calculator_$1.x"
  shasum -a 256 "$BAZEL_BIN/tests/core/go_library/reproducible_x/calculator.x" > "$tmp_output/calculator_$1.x.sha256"
  printf "\n\n\n"
}

build_and_show_hash "original"

echo "Changing the source code"
sed -i 's/division by zero is not allowed/division by zero is not allowed - modified/g' tests/core/go_library/reproducible_x/calculator.go

build_and_show_hash "modified"

echo "Comparing the hashes..."
if diff "$tmp_output/calculator_original.x.sha256" "$tmp_output/calculator_modified.x.sha256"; then
  echo "Hashes are the same. Output is reproducible"
else
  echo
  echo
  echo "Hashes are different"
  echo "Export archives are saved in $tmp_output"
  echo
  ls -l "$tmp_output"
  exit 1
fi
