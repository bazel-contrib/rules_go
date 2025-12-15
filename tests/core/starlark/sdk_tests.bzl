load("@bazel_skylib//lib:unittest.bzl", "asserts", "unittest")
load("//go/private:sdk.bzl", "go_toolchains_single_definition")

def _go_toolchains_single_definition_with_version_test(ctx):
    env = unittest.begin(ctx)

    result = go_toolchains_single_definition(
        ctx = None,
        prefix = "123_prefix_",
        goos = "linux",
        goarch = "amd64",
        sdk_repo = "sdk_repo",
        sdk_type = "download",
        sdk_version = "1.20.2rc1",
    )
    asserts.equals(env, [], result.loads)
    asserts.equals(env, [
        """
_123_PREFIX_MAJOR_VERSION = "1"
_123_PREFIX_MINOR_VERSION = "20"
_123_PREFIX_PATCH_VERSION = "2"
_123_PREFIX_PRERELEASE_SUFFIX = "rc1"
""",
        """declare_bazel_toolchains(
    prefix = "123_prefix_",
    go_toolchain_repo = "@sdk_repo",
    exec_goarch = "amd64",
    exec_goos = "linux",
    major = _123_PREFIX_MAJOR_VERSION,
    minor = _123_PREFIX_MINOR_VERSION,
    patch = _123_PREFIX_PATCH_VERSION,
    prerelease = _123_PREFIX_PRERELEASE_SUFFIX,
    sdk_name = "sdk_repo",
    sdk_type = "download",
)
""",
    ], result.chunks)

    return unittest.end(env)

go_toolchains_single_definition_with_version_test = unittest.make(_go_toolchains_single_definition_with_version_test)

def _go_toolchains_single_definition_without_version_test(ctx):
    env = unittest.begin(ctx)

    result = go_toolchains_single_definition(
        ctx = None,
        prefix = "123_prefix_",
        goos = "linux",
        goarch = "amd64",
        sdk_repo = "sdk_repo",
        sdk_type = "download",
        sdk_version = None,
    )
    asserts.equals(env, ["""load(
    "@sdk_repo//:version.bzl",
    _123_PREFIX_MAJOR_VERSION = "MAJOR_VERSION",
    _123_PREFIX_MINOR_VERSION = "MINOR_VERSION",
    _123_PREFIX_PATCH_VERSION = "PATCH_VERSION",
    _123_PREFIX_PRERELEASE_SUFFIX = "PRERELEASE_SUFFIX",
)
"""], result.loads)
    asserts.equals(env, [
        """declare_bazel_toolchains(
    prefix = "123_prefix_",
    go_toolchain_repo = "@sdk_repo",
    exec_goarch = "amd64",
    exec_goos = "linux",
    major = _123_PREFIX_MAJOR_VERSION,
    minor = _123_PREFIX_MINOR_VERSION,
    patch = _123_PREFIX_PATCH_VERSION,
    prerelease = _123_PREFIX_PRERELEASE_SUFFIX,
    sdk_name = "sdk_repo",
    sdk_type = "download",
)
""",
    ], result.chunks)

    return unittest.end(env)

go_toolchains_single_definition_without_version_test = unittest.make(_go_toolchains_single_definition_without_version_test)

def sdk_test_suite():
    unittest.suite(
        "sdk_tests",
        go_toolchains_single_definition_with_version_test,
        go_toolchains_single_definition_without_version_test,
    )
