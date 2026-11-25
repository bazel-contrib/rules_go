"""Check that repository-only SDKs do not change default and explicitly selected SDKs."""

load("@bazel_skylib//lib:unittest.bzl", "analysistest", "asserts")

_SdkInfo = provider(doc = "The SDK chosen by native toolchain resolution.", fields = ["sdk"])
_TOOLCHAIN = "@io_bazel_rules_go//go:toolchain"
_SDK_VERSION = str(Label("@io_bazel_rules_go//go/toolchain:sdk_version"))
_SDK_NAME = str(Label("@io_bazel_rules_go//go/toolchain:sdk_name"))

def _probe_impl(ctx):
    return [_SdkInfo(sdk = ctx.toolchains[_TOOLCHAIN].sdk)]

sdk_probe = rule(implementation = _probe_impl, toolchains = [_TOOLCHAIN])

def _selection_test_impl(ctx):
    env = analysistest.begin(ctx)
    sdk = analysistest.target_under_test(env)[_SdkInfo].sdk
    asserts.equals(env, ctx.attr.expected_version, sdk.version)
    for name in ["repository_only_download", "repository_only_from_file"]:
        duplicate = Label("@{}//:ROOT".format(name))
        asserts.false(env, sdk.root_file.owner == duplicate)
    return analysistest.end(env)

default_sdk_test = analysistest.make(
    _selection_test_impl,
    attrs = {"expected_version": attr.string(mandatory = True)},
    config_settings = {_SDK_VERSION: "", _SDK_NAME: ""},
)

alternate_sdk_test = analysistest.make(
    _selection_test_impl,
    attrs = {"expected_version": attr.string(mandatory = True)},
    config_settings = {_SDK_VERSION: "1.24.0", _SDK_NAME: ""},
)
