load("@bazel_skylib//lib:unittest.bzl", "analysistest", "asserts")
load("//go/tools/gopackagesdriver:aspect.bzl", "GoPkgInfo", "go_pkg_info_aspect")

def _json_files(target):
    return [f.basename for f in target[OutputGroupInfo].go_pkg_driver_json_file.to_list()]

def _package_driver_pkg_json_test_impl(ctx):
    env = analysistest.begin(ctx)

    json_files = _json_files(analysistest.target_under_test(env))
    for want in ctx.attr.want_json_files:
        asserts.true(env, want in json_files, "{} does not contain {}".format(json_files, want))

    return analysistest.end(env)

package_driver_pkg_json_test = analysistest.make(
    _package_driver_pkg_json_test_impl,
    attrs = {
        "want_json_files": attr.string_list(),
    },
    extra_target_under_test_aspects = [go_pkg_info_aspect],
)

def _package_driver_skips_non_go_test_impl(ctx):
    env = analysistest.begin(ctx)

    target_under_test = analysistest.target_under_test(env)
    asserts.false(env, GoPkgInfo in target_under_test, "go_pkg_info_aspect was applied to a non-Go target")

    return analysistest.end(env)

# Configures the target for a platform with no Go toolchain, so analysis fails
# if the aspect resolves one for it.
package_driver_skips_non_go_test = analysistest.make(
    _package_driver_skips_non_go_test_impl,
    extra_target_under_test_aspects = [go_pkg_info_aspect],
    config_settings = {
        "//command_line_option:platforms": str(Label("//tests/core/starlark/packagedriver/fixtures/non_go:no_go_toolchain")),
    },
)

def _test_package_driver():
    package_driver_pkg_json_test(
        name = "package_driver_should_return_pkg_json_for_go_test",
        target_under_test = "//tests/core/starlark/packagedriver/fixtures/c:go_default_test",
        want_json_files = ["go_default_test.pkg.json"],
    )

    package_driver_pkg_json_test(
        name = "package_driver_should_return_pkg_json_for_go_proto_library",
        target_under_test = "//tests/core/starlark/packagedriver/fixtures/proto:uses_proto",
        want_json_files = [
            "uses_proto.pkg.json",
            "foo_go_proto.pkg.json",
        ],
    )

    package_driver_skips_non_go_test(
        name = "package_driver_should_skip_non_go_targets",
        target_under_test = "//tests/core/starlark/packagedriver/fixtures/non_go:non_go",
    )

def package_driver_suite(name):
    _test_package_driver()

    native.test_suite(
        name = name,
        tests = [
            ":package_driver_should_return_pkg_json_for_go_test",
            ":package_driver_should_return_pkg_json_for_go_proto_library",
            ":package_driver_should_skip_non_go_targets",
        ],
    )
