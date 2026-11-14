load("@bazel_skylib//lib:unittest.bzl", "asserts", "unittest")
load("//go/private:context.bzl", "filter_cgo_compiler_options", "matches_scope")

def _matches_scope_test(ctx):
    env = unittest.begin(ctx)

    # With --enable_bzlmod, the apparent repository names used below need to be valid.
    asserts.true(env, matches_scope(Label("//some/pkg:bar"), "all"))
    asserts.true(env, matches_scope(Label("@com_google_protobuf//some/pkg:bar"), "all"))

    asserts.true(env, matches_scope(Label("//:bar"), Label("//:__pkg__")))
    asserts.false(env, matches_scope(Label("//some:bar"), Label("//:__pkg__")))
    asserts.false(env, matches_scope(Label("//some/pkg:bar"), Label("//:__pkg__")))
    asserts.false(env, matches_scope(Label("@com_google_protobuf//:bar"), Label("//:__pkg__")))
    asserts.false(env, matches_scope(Label("@com_google_protobuf//some:bar"), Label("//:__pkg__")))
    asserts.false(env, matches_scope(Label("@com_google_protobuf//some/pkg:bar"), Label("//:__pkg__")))

    asserts.false(env, matches_scope(Label("//:bar"), Label("//some:__pkg__")))
    asserts.true(env, matches_scope(Label("//some:bar"), Label("//some:__pkg__")))
    asserts.false(env, matches_scope(Label("//some/pkg:bar"), Label("//some:__pkg__")))
    asserts.false(env, matches_scope(Label("@com_google_protobuf//:bar"), Label("//some:__pkg__")))
    asserts.false(env, matches_scope(Label("@com_google_protobuf//some:bar"), Label("//some:__pkg__")))
    asserts.false(env, matches_scope(Label("@com_google_protobuf//some/pkg:bar"), Label("//some:__pkg__")))

    asserts.true(env, matches_scope(Label("//:bar"), Label("//:__subpackages__")))
    asserts.true(env, matches_scope(Label("//some:bar"), Label("//:__subpackages__")))
    asserts.true(env, matches_scope(Label("//some/pkg:bar"), Label("//:__subpackages__")))
    asserts.false(env, matches_scope(Label("@com_google_protobuf//:bar"), Label("//:__subpackages__")))
    asserts.false(env, matches_scope(Label("@com_google_protobuf//some:bar"), Label("//:__subpackages__")))
    asserts.false(env, matches_scope(Label("@com_google_protobuf//some/pkg:bar"), Label("//:__subpackages__")))

    asserts.false(env, matches_scope(Label("//:bar"), Label("//some:__subpackages__")))
    asserts.true(env, matches_scope(Label("//some:bar"), Label("//some:__subpackages__")))
    asserts.true(env, matches_scope(Label("//some/pkg:bar"), Label("//some:__subpackages__")))
    asserts.false(env, matches_scope(Label("@com_google_protobuf//:bar"), Label("//some:__subpackages__")))
    asserts.false(env, matches_scope(Label("@com_google_protobuf//some:bar"), Label("//some:__subpackages__")))
    asserts.false(env, matches_scope(Label("@com_google_protobuf//some/pkg:bar"), Label("//some:__subpackages__")))

    return unittest.end(env)

matches_scope_test = unittest.make(_matches_scope_test)

def _filter_cgo_compiler_options_test(ctx):
    env = unittest.begin(ctx)

    options = [
        "-O2",
        "-DKEEP=1",
        "-fsanitize=cfi",
        "-fsanitize=cfi-cast-strict",
        "-fsanitize=cfi-derived-cast",
        "-fsanitize=cfi-icall",
        "-fsanitize=cfi-mfcall",
        "-fsanitize=cfi-nvcall",
        "-fsanitize=cfi-unrelated-cast",
        "-fsanitize=cfi-vcall",
        "-fsanitize=kcfi",
        "-fsanitize=address",
        "-fsanitize=undefined",
        "-fsanitize=cfi,undefined",
        "-fsanitize=cfi-cast-strict,undefined",
        "-fsanitize=address,cfi",
        "-fsanitize=kcfi,undefined",
        "-fsanitize=address,undefined",
    ]
    asserts.equals(
        env,
        [
            "-O2",
            "-DKEEP=1",
            "-fsanitize=kcfi",
            "-fsanitize=address",
            "-fsanitize=undefined",
            "-fsanitize=undefined",
            "-fsanitize=undefined",
            "-fsanitize=address",
            "-fsanitize=kcfi,undefined",
            "-fsanitize=address,undefined",
        ],
        filter_cgo_compiler_options(options),
    )

    return unittest.end(env)

filter_cgo_compiler_options_test = unittest.make(_filter_cgo_compiler_options_test)

def context_test_suite():
    """Creates the test targets and test suite for context.bzl tests."""
    unittest.suite(
        "context_tests",
        matches_scope_test,
        filter_cgo_compiler_options_test,
    )
