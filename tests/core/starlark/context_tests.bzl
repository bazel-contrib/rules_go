load("@bazel_skylib//lib:unittest.bzl", "analysistest", "asserts", "unittest")
load("//go:def.bzl", "go_context", "go_rule")
load("//go/private:context.bzl", "CPP_TOOLCHAIN_TYPE", "matches_scope")

_GO_TOOLCHAIN_TYPE = Label("//go:toolchain")
_SHELL_TOOLCHAIN_TYPE = Label("@bazel_tools//tools/sh:toolchain_type")

_GoRuleContextInfo = provider(
    doc = "Records the configuration and toolchains supplied by go_rule.",
    fields = [
        "custom_value",
        "has_apple_fragment",
        "has_cpp_fragment",
        "has_cpp_toolchain",
        "has_go_toolchain",
        "has_java_fragment",
        "has_shell_toolchain",
        "race",
    ],
)

def _go_rule_context_impl(ctx):
    go = go_context(ctx)
    return [_GoRuleContextInfo(
        custom_value = ctx.attr.custom_value,
        has_apple_fragment = hasattr(ctx.fragments, "apple"),
        has_cpp_fragment = hasattr(ctx.fragments, "cpp"),
        has_cpp_toolchain = CPP_TOOLCHAIN_TYPE in ctx.toolchains,
        has_go_toolchain = _GO_TOOLCHAIN_TYPE in ctx.toolchains,
        has_java_fragment = hasattr(ctx.fragments, "java"),
        has_shell_toolchain = _SHELL_TOOLCHAIN_TYPE in ctx.toolchains,
        race = go.mode.race,
    )]

_GO_RULE_CONTEXT_ATTRS = {
    "custom_value": attr.string(mandatory = True),
    "_go_context_data": attr.label(default = "//:go_context_data"),
}

_go_rule_context = go_rule(
    implementation = _go_rule_context_impl,
    attrs = _GO_RULE_CONTEXT_ATTRS,
    fragments = ["java"],
    toolchains = [
        "@io_bazel_rules_go//go:toolchain",
        config_common.toolchain_type(_SHELL_TOOLCHAIN_TYPE, mandatory = False),
    ],
)

_go_rule_positional_context = go_rule(
    _go_rule_context_impl,
    attrs = _GO_RULE_CONTEXT_ATTRS,
    fragments = ["java"],
    toolchains = [config_common.toolchain_type(_SHELL_TOOLCHAIN_TYPE, mandatory = False)],
)

_unwrapped_go_context = rule(
    implementation = _go_rule_context_impl,
    attrs = _GO_RULE_CONTEXT_ATTRS,
    toolchains = ["@io_bazel_rules_go//go:toolchain"],
)

def _go_rule_context_test(ctx):
    env = analysistest.begin(ctx)
    info = analysistest.target_under_test(env)[_GoRuleContextInfo]

    asserts.equals(env, "caller attribute", info.custom_value)
    asserts.true(env, info.has_apple_fragment, "go_rule must declare the Apple fragment")
    asserts.true(env, info.has_cpp_fragment, "go_rule must declare the C++ fragment")
    asserts.true(env, info.has_cpp_toolchain, "go_rule must declare the optional C++ toolchain")
    asserts.true(env, info.has_go_toolchain, "go_rule must declare the Go toolchain")
    asserts.true(env, info.has_java_fragment, "go_rule must preserve caller-declared fragments")
    asserts.true(env, info.has_shell_toolchain, "go_rule must preserve caller-declared toolchains")
    asserts.true(env, info.race, "go_rule must preserve race-mode Go configuration")

    return analysistest.end(env)

go_rule_context_test = analysistest.make(
    _go_rule_context_test,
    config_settings = {str(Label("//go/config:race")): True},
)

def _missing_go_rule_test(ctx):
    env = analysistest.begin(ctx)

    asserts.expect_failure(env, "Define this rule with go_rule(...) instead of rule(...)")

    return analysistest.end(env)

missing_go_rule_test = analysistest.make(
    _missing_go_rule_test,
    expect_failure = True,
    config_settings = {str(Label("//go/config:race")): True},
)

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

def context_test_suite():
    """Creates the test targets and test suite for context.bzl tests."""
    _go_rule_context(
        name = "go_rule_context",
        custom_value = "caller attribute",
        tags = ["manual"],
    )
    _go_rule_positional_context(
        name = "go_rule_positional_context",
        custom_value = "caller attribute",
        tags = ["manual"],
    )
    _unwrapped_go_context(
        name = "unwrapped_go_context",
        custom_value = "caller attribute",
        tags = ["manual"],
    )
    go_rule_context_test(
        name = "go_rule_context_test",
        target_under_test = ":go_rule_context",
    )
    go_rule_context_test(
        name = "go_rule_positional_context_test",
        target_under_test = ":go_rule_positional_context",
    )
    missing_go_rule_test(
        name = "missing_go_rule_test",
        target_under_test = ":unwrapped_go_context",
    )

    unittest.suite(
        "context_tests",
        matches_scope_test,
    )
