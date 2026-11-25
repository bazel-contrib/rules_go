#!/usr/bin/env bash

set -eu

# The value of GOCACHE/GOPATH are determined from HOME.
# We place them in the execroot to avoid dependency on `mktemp` and because we don't know
# a safe scratch space on all systems. Note that HOME must be an absolute path, otherwise the
# Go toolchain will write some outputs to the wrong place and the result will be uncacheable.
# We include an output path of this action to prevent collisions with anything else,
# including differently configured versions of the same target, under an unsandboxed strategy.

export HOME="${PWD}/_go_tool_binary-fake-home-${1//\\//_}"
trap "${GO_BINARY} clean -cache" EXIT

# Resolve GO_BINARY to the real SDK so that `go` derives GOROOT from the
# extracted repository rather than from sandbox symlinks. Linux already does
# this implicitly (os.Executable reads /proc/self/exe), but macOS uses the
# unresolved path, and go:embed rejects symlinked files ("cannot embed
# irregular file"), e.g. in SDKs that embed files into cmd/pack's deps.
while [ -L "${GO_BINARY}" ]; do
  link="$(readlink "${GO_BINARY}")"
  case "${link}" in
    /*) GO_BINARY="${link}" ;;
    *) GO_BINARY="$(dirname "${GO_BINARY}")/${link}" ;;
  esac
done

# We do not use -a here as the cache drastically reduces the time spent
# on the subsequent go build invocations (roughly 50% faster).

"${GO_BINARY}" build -trimpath -ldflags="-buildid=\"\" ${LD_FLAGS}" -o "$1" cmd/pack

"${GO_BINARY}" build -trimpath -ldflags="-buildid=\"\" ${LD_FLAGS}" -o "$2" cmd/covdata
shift 2

"${GO_BINARY}" build -trimpath -ldflags="-buildid=\"\" ${LD_FLAGS}" -o "$@"
