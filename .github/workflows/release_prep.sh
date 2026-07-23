#!/usr/bin/env bash

set -o errexit -o nounset -o pipefail

# Set by GH actions, see
# https://docs.github.com/en/actions/learn-github-actions/environment-variables#default-environment-variables
TAG=${GITHUB_REF_NAME}
ARCHIVE="rules_go-${TAG}.zip"
git archive --format=zip --output="$ARCHIVE" "$TAG"
SHA=$(shasum -a 256 "$ARCHIVE" | awk '{print $1}')
# The latest stable Go version, used in the go_register_toolchains boilerplate.
GO_VERSION=$(curl --silent --show-error --fail 'https://go.dev/dl/?mode=json' | jq --raw-output '.[0].version' | sed 's/^go//')

cat << EOF
## \`MODULE.bazel\` code

\`\`\`
bazel_dep(name = "rules_go", version = "${TAG:1}")

go_sdk = use_extension("@rules_go//go:extensions.bzl", "go_sdk")
go_sdk.from_file(go_mod = "//:go.mod")
\`\`\`

## \`WORKSPACE\` code

\`\`\`
load("@bazel_tools//tools/build_defs/repo:http.bzl", "http_archive")

http_archive(
    name = "io_bazel_rules_go",
    sha256 = "${SHA}",
    urls = [
        "https://github.com/bazel-contrib/rules_go/releases/download/${TAG}/${ARCHIVE}",
    ],
)

load("@io_bazel_rules_go//go:deps.bzl", "go_register_toolchains", "go_rules_dependencies")

go_rules_dependencies()

go_register_toolchains(version = "${GO_VERSION}")

# Create the host platform repository transitively required by rules_go.
load("@bazel_tools//tools/build_defs/repo:utils.bzl", "maybe")
load("@platforms//host:extension.bzl", "host_platform_repo")

maybe(
    host_platform_repo,
    name = "host_platform",
)
\`\`\`
EOF
