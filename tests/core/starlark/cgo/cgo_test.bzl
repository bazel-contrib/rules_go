load("@bazel_skylib//lib:unittest.bzl", "analysistest", "asserts")
load("@rules_cc//cc:cc_toolchain_config_lib.bzl", "feature", "tool_path")  # buildifier: disable=deprecated-function
load("@rules_cc//cc:defs.bzl", "cc_toolchain")
load("@rules_cc//cc/common:cc_common.bzl", "cc_common")
load("@io_bazel_rules_go//go:def.bzl", "go_binary", "go_cross_binary")

def _test_cc_config_impl(ctx):
    tool_paths = [
        tool_path(name = name, path = "/bin/false")
        for name in [
            "ar",
            "cpp",
            "dwp",
            "gcc",
            "gcov",
            "ld",
            "nm",
            "objcopy",
            "objdump",
            "strip",
        ]
    ]

    return cc_common.create_cc_toolchain_config_info(
        ctx = ctx,
        toolchain_identifier = "runtime-libs-test-toolchain",
        host_system_name = "local",
        target_system_name = "local",
        target_cpu = "local",
        target_libc = "local",
        compiler = "gcc",
        abi_version = "local",
        abi_libc_version = "local",
        tool_paths = tool_paths,
        features = [
            feature(name = "static_link_cpp_runtimes", enabled = True),
        ],
    )

test_cc_config = rule(
    implementation = _test_cc_config_impl,
    provides = [CcToolchainConfigInfo],
)

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

def _runtime_lib_inputs_test_impl(ctx):
    env = analysistest.begin(ctx)
    actions = analysistest.target_actions(env)
    link_actions = [action for action in actions if action.mnemonic == "GoLink"]
    asserts.equals(env, 1, len(link_actions), "expected exactly one GoLink action")

    inputs = link_actions[0].inputs.to_list()
    for expected in ctx.attr.expected_inputs:
        asserts.true(
            env,
            any([input.path.endswith("/" + expected) for input in inputs]),
            "expected '{}' to be in inputs: '{}'".format(expected, inputs),
        )
    for unexpected in ctx.attr.unexpected_inputs:
        asserts.false(
            env,
            any([input.path.endswith("/" + unexpected) for input in inputs]),
            "did not expect '{}' to be in inputs: '{}'".format(unexpected, inputs),
        )

    compile_actions = [action for action in actions if action.mnemonic == "GoCompilePkg"]
    asserts.equals(env, 1, len(compile_actions), "expected exactly one GoCompilePkg action")
    compile_argv = " ".join(compile_actions[0].argv)
    for expected in ctx.attr.expected_linkopts:
        asserts.true(
            env,
            expected in compile_argv,
            "expected '{}' to be in compile argv: '{}'".format(expected, compile_argv),
        )
    for unexpected in ctx.attr.unexpected_linkopts:
        asserts.false(
            env,
            unexpected in compile_argv,
            "did not expect '{}' to be in compile argv: '{}'".format(unexpected, compile_argv),
        )

    return analysistest.end(env)

runtime_lib_inputs_test = analysistest.make(
    _runtime_lib_inputs_test_impl,
    attrs = {
        "expected_inputs": attr.string_list(),
        "expected_linkopts": attr.string_list(),
        "unexpected_inputs": attr.string_list(),
        "unexpected_linkopts": attr.string_list(),
    },
    config_settings = {
        "//command_line_option:extra_toolchains": str(Label("//tests/core/starlark/cgo:runtime_libs_test_cc_toolchain")),
    },
)

def cgo_test_suite():
    test_cc_config(
        name = "runtime_libs_test_cc_toolchain_config",
    )

    cc_toolchain(
        name = "runtime_libs_test_cc_toolchain_impl",
        all_files = ":empty",
        compiler_files = ":empty",
        dwp_files = ":empty",
        dynamic_runtime_lib = ":dummy.so",
        linker_files = ":empty",
        objcopy_files = ":empty",
        static_runtime_lib = ":dummy.a",
        strip_files = ":empty",
        supports_param_files = 0,
        toolchain_config = ":runtime_libs_test_cc_toolchain_config",
        toolchain_identifier = "runtime-libs-test-toolchain",
    )

    native.toolchain(
        name = "runtime_libs_test_cc_toolchain",
        toolchain = ":runtime_libs_test_cc_toolchain_impl",
        toolchain_type = "@bazel_tools//tools/cpp:toolchain_type",
    )

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

    go_binary(
        name = "runtime_libs_static_binary",
        srcs = [
            "runtime_libs.cc",
            "runtime_libs.go",
        ],
        cgo = True,
        pure = "off",
        tags = ["manual"],
    )

    runtime_lib_inputs_test(
        name = "static_runtime_lib_inputs_test",
        expected_inputs = ["dummy.a"],
        expected_linkopts = ["dummy.a"],
        target_under_test = ":runtime_libs_static_binary",
        unexpected_inputs = ["dummy.so"],
        unexpected_linkopts = ["dummy.so"],
    )

    go_binary(
        name = "runtime_libs_dynamic_binary",
        srcs = [
            "runtime_libs.cc",
            "runtime_libs.go",
        ],
        cgo = True,
        linkmode = "c-shared",
        pure = "off",
        tags = ["manual"],
    )

    runtime_lib_inputs_test(
        name = "dynamic_runtime_lib_inputs_test",
        expected_inputs = ["dummy.so"],
        expected_linkopts = ["dummy.so"],
        target_under_test = ":runtime_libs_dynamic_binary",
        unexpected_inputs = ["dummy.a"],
        unexpected_linkopts = ["dummy.a"],
    )

    """Creates the test targets and test suite for cgo.bzl tests."""
    native.test_suite(
        name = "cgo_tests",
        tests = [
            ":dynamic_runtime_lib_inputs_test",
            ":missing_cc_toolchain_explicit_pure_off_test",
            ":static_runtime_lib_inputs_test",
        ],
    )
