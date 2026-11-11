load("@bazel_skylib//lib:unittest.bzl", "analysistest", "asserts")
load("@package_metadata//providers:package_metadata_info.bzl", "PackageMetadataInfo")
load("@package_metadata//rules:package_metadata.bzl", "package_metadata")
load("//go:def.bzl", "GoInfo", "go_library")
load("//go/private:context.bzl", "go_context", "go_rule", "new_go_info")

def _go_package_metadata_impl(ctx):
    go = go_context(ctx)
    return [new_go_info(
        go,
        struct(package_metadata = ctx.attr.metadata),
    )]

go_package_metadata = go_rule(
    implementation = _go_package_metadata_impl,
    attrs = {
        "metadata": attr.label_list(providers = [PackageMetadataInfo]),
    },
)

def _package_metadata_test_impl(ctx):
    env = analysistest.begin(ctx)
    metadata = analysistest.target_under_test(env)[GoInfo]._package_metadata
    asserts.equals(env, "cmp_package_metadata.package-metadata.json", metadata.basename)
    return analysistest.end(env)

package_metadata_test = analysistest.make(_package_metadata_test_impl)

def _applicable_licenses_test_impl(ctx):
    env = analysistest.begin(ctx)
    metadata = analysistest.target_under_test(env)[GoInfo]._package_metadata
    asserts.equals(env, "sync_package_metadata.package-metadata.json", metadata.basename)
    return analysistest.end(env)

applicable_licenses_test = analysistest.make(_applicable_licenses_test_impl)

def module_info_test_suite():
    package_metadata(
        name = "cmp_package_metadata",
        purl = "pkg:golang/github.com/google/go-cmp@v0.6.0",
    )

    # package_metadata is not a built-in rule attribute in older supported
    # Bazel versions, so use a test rule to exercise new_go_info directly.
    go_package_metadata(
        name = "package_metadata_library",
        metadata = [":cmp_package_metadata"],
        tags = ["manual"],
    )

    package_metadata_test(
        name = "package_metadata_test",
        target_under_test = ":package_metadata_library",
    )

    package_metadata(
        name = "sync_package_metadata",
        purl = "pkg:golang/golang.org/x/sync@v0.8.0",
    )

    go_library(
        name = "applicable_licenses_library",
        applicable_licenses = [":sync_package_metadata"],
        tags = ["manual"],
    )

    applicable_licenses_test(
        name = "applicable_licenses_test",
        target_under_test = ":applicable_licenses_library",
    )
