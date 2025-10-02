#!/usr/bin/env bash

# --- begin runfiles.bash initialization v3 ---
# Copy-pasted from the Bazel Bash runfiles library v3.
set -uo pipefail; set +e; f=bazel_tools/tools/bash/runfiles/runfiles.bash
source "${RUNFILES_DIR:-/dev/null}/$f" 2>/dev/null || \
  source "$(grep -sm1 "^$f " "${RUNFILES_MANIFEST_FILE:-/dev/null}" | cut -f2- -d' ')" 2>/dev/null || \
  source "$0.runfiles/$f" 2>/dev/null || \
  source "$(grep -sm1 "^$f " "$0.runfiles_manifest" | cut -f2- -d' ')" 2>/dev/null || \
  source "$(grep -sm1 "^$f " "$0.exe.runfiles_manifest" | cut -f2- -d' ')" 2>/dev/null || \
  { echo>&2 "ERROR: cannot find $f"; exit 1; }; f=; set -e
# --- end runfiles.bash initialization v3 ---

GO_TOOL=$(rlocation "$GO_TOOL_RLOCATION")
# Set runfiles variables for subprocess.
runfiles_export_envvars
# Simulate a bazel run environment.
export BUILD_WORKING_DIRECTORY=$(pwd)
[[ "$("$GO_TOOL" version)" =~ ^go ]]
[[ "$("$GO_TOOL" env GOPRIVATE)" == example.com ]]
# Test that explicit env vars override `go_env` settings.
[[ "$(GOPRIVATE=foo.com "$GO_TOOL" env GOPRIVATE)" == foo.com ]]
