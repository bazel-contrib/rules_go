workspace(name = "io_bazel_rules_go")

load("@bazel_tools//tools/build_defs/repo:http.bzl", "http_archive")
load("@io_bazel_rules_go//go:deps.bzl", "go_register_nogo", "go_register_toolchains", "go_rules_dependencies")

# The non-polyfill version of this is needed by rules_proto below.
http_archive(
    name = "bazel_features",
    sha256 = "9390b391a68d3b24aef7966bce8556d28003fe3f022a5008efc7807e8acaaf1a",
    strip_prefix = "bazel_features-1.36.0",
    url = "https://github.com/bazel-contrib/bazel_features/releases/download/v1.36.0/bazel_features-v1.36.0.tar.gz",
)

load("@bazel_features//:deps.bzl", "bazel_features_deps")

bazel_features_deps()

go_rules_dependencies()

go_register_toolchains(version = "1.24.0")

go_register_nogo(
    nogo = "@//internal:nogo",
)

# Used by //tests:buildifier_test.
http_archive(
    name = "com_github_bazelbuild_buildtools",
    sha256 = "05c3c3602d25aeda1e9dbc91d3b66e624c1f9fdadf273e5480b489e744ca7269",
    strip_prefix = "buildtools-6.4.0",
    # latest, as of 2023-11-17
    urls = ["https://github.com/bazelbuild/buildtools/archive/refs/tags/v6.4.0.tar.gz"],
)

load("@bazel_ci_rules//:rbe_repo.bzl", "rbe_preconfig")

# Creates a default toolchain config for RBE.
# Use this as is if you are using the rbe_ubuntu16_04 container,
# otherwise refer to RBE docs.
rbe_preconfig(
    name = "buildkite_config",
    toolchain = "ubuntu2204",
)

http_archive(
    name = "bazel_gazelle",
    sha256 = "b760f7fe75173886007f7c2e616a21241208f3d90e8657dc65d36a771e916b6a",
    urls = [
        "https://mirror.bazel.build/github.com/bazelbuild/bazel-gazelle/releases/download/v0.39.1/bazel-gazelle-v0.39.1.tar.gz",
        "https://github.com/bazelbuild/bazel-gazelle/releases/download/v0.39.1/bazel-gazelle-v0.39.1.tar.gz",
    ],
)

load("@gazelle//:deps.bzl", "gazelle_dependencies", "go_repository")

gazelle_dependencies(go_sdk = "go_sdk")

go_repository(
    name = "com_github_google_go_github_v36",
    importpath = "github.com/google/go-github/v36",
    sum = "h1:ndCzM616/oijwufI7nBRa+5eZHLldT+4yIB68ib5ogs=",
    version = "v36.0.0",
)

go_repository(
    name = "com_github_google_go_querystring",
    importpath = "github.com/google/go-querystring",
    sum = "h1:AnCroh3fv4ZBgVIf1Iwtovgjaw/GiKJo8M8yD/fhyJ8=",
    version = "v1.1.0",
)

go_repository(
    name = "org_golang_x_oauth2",
    importpath = "golang.org/x/oauth2",
    sum = "h1:Lh8GPgSKBfWSwFvtuWOfeI3aAAnbXTSutYxJiOJFgIw=",
    version = "v0.6.0",
)

load("@io_bazel_rules_go//tests/legacy/test_chdir:remote.bzl", "test_chdir_remote")

test_chdir_remote()

load("@io_bazel_rules_go//tests/integration/popular_repos:popular_repos.bzl", "popular_repos")

popular_repos()

load(
    "@build_bazel_apple_support//lib:repositories.bzl",
    "apple_support_dependencies",
)

apple_support_dependencies()

# For testing the compatibility with a hermetic cc toolchain. Users should not have to enable it.
http_archive(
    name = "hermetic_cc_toolchain",
    sha256 = "bd2234acd0837251361be3270d7d3ce599b418be123d902d84762302e31a3014",
    strip_prefix = "hermetic_cc_toolchain-13c904dce0cb9b6d07f0d557e6ce3cf7013a562e",
    urls = ["https://github.com/uber/hermetic_cc_toolchain/archive/13c904dce0cb9b6d07f0d557e6ce3cf7013a562e.zip"],
)

load("@hermetic_cc_toolchain//toolchain:defs.bzl", zig_toolchains = "toolchains")

zig_toolchains(
    host_platform_sha256 = {
        "linux-aarch64": "12be476ed53c219507e77737dbb7f2a77b280760b8acbc6ba2eaaeb42b7d145e",
        "linux-x86_64": "1b1c115c4ccbdc215cc3b07833c7957336d9f5fff816f97e5cafee556a9d8be8",
        "macos-aarch64": "3943612c560dd066fba5698968317a146a0f585f6cdaa1e7c1df86685c7c4eaf",
        "macos-x86_64": "0c89e5d934ecbf9f4d2dea6e3b8dfcc548a3d4184a856178b3db74e361031a2b",
    },
    version = "0.11.0-dev.3886+0c1bfe271",
)
