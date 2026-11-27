load("@bazel_skylib//lib:unittest.bzl", "analysistest", "asserts", "unittest")
load("//go/private:sdk.bzl", "fips_snapshot_import_paths", "go_toolchains_single_definition")
load("//go/private/rules:sdk.bzl", "package_list")

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

def _fips_snapshot_import_paths_test(ctx):
    env = unittest.begin(ctx)

    asserts.equals(
        env,
        [
            "crypto/internal/fips140/v1.0.0-c2097c7c",
            "crypto/internal/fips140/v1.0.0-c2097c7c/aes",
            "crypto/internal/fips140/v1.0.0-c2097c7c/aes/gcm",
            "crypto/internal/fips140/v1.0.0-c2097c7c/sha256",
        ],
        fips_snapshot_import_paths(
            [
                "sha256/sha256.go",
                "aes/gcm/gcm_asm.go",
                "aes/aes.go",
                "aes/cbc.go",
                "cast.go",
            ],
            "v1.0.0-c2097c7c",
        ),
    )
    asserts.equals(env, [], fips_snapshot_import_paths([], "v1.0.0-c2097c7c"))

    return unittest.end(env)

fips_snapshot_import_paths_test = unittest.make(_fips_snapshot_import_paths_test)

def _package_list_fips_out_test_impl(ctx):
    env = analysistest.begin(ctx)

    contents = {
        action.outputs.to_list()[0].basename: action.content
        for action in analysistest.target_actions(env)
    }
    asserts.equals(env, "a/x\nb\n", contents["sdk_tests_fips_packages.txt"])
    asserts.equals(env, "a/x\nb\n", contents["sdk_tests_packages.txt"])

    return analysistest.end(env)

package_list_fips_out_test = analysistest.make(_package_list_fips_out_test_impl)

def sdk_test_suite():
    package_list(
        name = "sdk_tests_package_list",
        out = "sdk_tests_packages.txt",
        fips_out = "sdk_tests_fips_packages.txt",
        fips_packages = ["b", "a/x"],
        root_file = "sdk_tests.bzl",
        tags = ["manual"],
    )

    package_list_fips_out_test(
        name = "package_list_fips_out_test",
        target_under_test = ":sdk_tests_package_list",
    )

    unittest.suite(
        "sdk_tests",
        go_toolchains_single_definition_with_version_test,
        go_toolchains_single_definition_without_version_test,
        fips_snapshot_import_paths_test,
    )
