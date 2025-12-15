load("@bazel_skylib//lib:unittest.bzl", "analysistest", "asserts")
load("@io_bazel_rules_go//go:def.bzl", "go_binary", "go_cross_binary")

def _missing_cc_toolchain_explicit_pure_off_test(ctx):
    env = analysistest.begin(ctx)

    asserts.expect_failure(env, "has pure explicitly set to off, but no C++ toolchain could be found for its platform")

    return analysistest.end(env)

missing_cc_toolchain_explicit_pure_off_test = analysistest.make(
    _missing_cc_toolchain_explicit_pure_off_test,
    expect_failure = True,
    config_settings = {
        "//command_line_option:extra_toolchains": str(Label("//tests/core/starlark/cgo:fake_go_toolchain")),
    },
)

def cgo_test_suite():
    go_binary(
        name = "cross_impure",
        srcs = ["main.go"],
        pure = "off",
        tags = ["manual"],
    )

    go_cross_binary(
        name = "go_cross_impure_cgo",
        platform = ":platform_has_no_cc_toolchain",
        target = ":cross_impure",
        tags = ["manual"],
    )

    missing_cc_toolchain_explicit_pure_off_test(
        name = "missing_cc_toolchain_explicit_pure_off_test",
        target_under_test = ":go_cross_impure_cgo",
    )

    """Creates the test targets and test suite for cgo.bzl tests."""
    native.test_suite(
        name = "cgo_tests",
        tests = [":missing_cc_toolchain_explicit_pure_off_test"],
    )
