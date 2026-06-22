load("@bazel_skylib//lib:unittest.bzl", "analysistest", "asserts")
load("//go:def.bzl", "GoArchive", "go_binary", "go_library", "go_test")

GoArchiveAspectInfo = provider()

def _go_archive_aspect_impl(_target, _ctx):
    return [GoArchiveAspectInfo()]

_go_archive_aspect = aspect(
    implementation = _go_archive_aspect_impl,
    required_providers = [GoArchive],
)

def _go_archive_aspect_consumer_impl(ctx):
    if GoArchiveAspectInfo not in ctx.attr.dep:
        fail("GoArchive aspect was not applied to {}".format(ctx.attr.dep.label))

go_archive_aspect_consumer = rule(
    implementation = _go_archive_aspect_consumer_impl,
    attrs = {
        "dep": attr.label(aspects = [_go_archive_aspect]),
    },
)

def _required_provider_test_impl(ctx):
    env = analysistest.begin(ctx)
    return analysistest.end(env)

required_provider_test = analysistest.make(_required_provider_test_impl)

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
        name = "go_library",
        tags = ["manual"],
    )

    for rule_name in ["go_binary", "go_library", "go_test"]:
        go_archive_aspect_consumer(
            name = rule_name + "_go_archive_aspect_consumer",
            dep = ":" + rule_name,
            tags = ["manual"],
            testonly = True,
        )

        required_provider_test(
            name = rule_name + "_go_archive_required_provider_test",
            target_under_test = ":" + rule_name + "_go_archive_aspect_consumer",
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
