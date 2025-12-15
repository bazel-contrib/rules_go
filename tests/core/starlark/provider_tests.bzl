load("@bazel_skylib//lib:unittest.bzl", "analysistest", "asserts")
load("//go:def.bzl", "go_binary", "go_library", "go_test")

# go_binary and go_test targets must not be used as deps/embed attributes;
# their dependencies may be built in different modes, resulting in conflicts and opaque errors.
def _providers_test_impl(ctx):
    env = analysistest.begin(ctx)
    asserts.expect_failure(env, "does not have mandatory providers")
    return analysistest.end(env)

providers_test = analysistest.make(
    _providers_test_impl,
    expect_failure = True,
)

def provider_test_suite():
    go_binary(
        name = "go_binary",
        tags = ["manual"],
    )

    go_library(
        name = "lib_binary_deps",
        deps = [":go_binary"],
        tags = ["manual"],
    )

    providers_test(
        name = "go_binary_deps_test",
        target_under_test = ":lib_binary_deps",
    )

    go_library(
        name = "lib_binary_embed",
        embed = [":go_binary"],
        tags = ["manual"],
    )

    providers_test(
        name = "go_binary_embed_test",
        target_under_test = ":lib_binary_embed",
    )

    go_test(
        name = "go_test",
        tags = ["manual"],
    )

    go_library(
        name = "lib_test_deps",
        deps = [":go_test"],
        tags = ["manual"],
    )

    providers_test(
        name = "go_test_deps_test",
        target_under_test = ":lib_test_deps",
    )

    go_library(
        name = "lib_embed_test",
        embed = [":go_test"],
        tags = ["manual"],
    )

    providers_test(
        name = "go_test_embed_test",
        target_under_test = ":lib_embed_test",
    )
