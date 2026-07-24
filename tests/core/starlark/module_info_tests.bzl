load("@bazel_skylib//lib:unittest.bzl", "analysistest", "asserts")
load("@package_metadata//rules:package_metadata.bzl", "package_metadata")
load("//go/private:context.bzl", "package_metadata_file_from_metadata")

PackageMetadataProbeInfo = provider()

def _package_metadata_probe_impl(ctx):
    metadata = package_metadata_file_from_metadata(
        Label(ctx.attr.module_label),
        getattr(ctx.attr, "package_metadata", ()),
        getattr(ctx.attr, "applicable_licenses", ()),
    )
    return [PackageMetadataProbeInfo(
        basename = metadata.basename if metadata else "",
    )]

package_metadata_probe = rule(
    implementation = _package_metadata_probe_impl,
    attrs = {
        "module_label": attr.string(mandatory = True),
        "package_metadata": attr.label_list(),
    },
)

def _package_metadata_test_impl(ctx):
    env = analysistest.begin(ctx)
    info = analysistest.target_under_test(env)[PackageMetadataProbeInfo]
    asserts.equals(env, "cmp_package_metadata.package-metadata.json", info.basename)
    return analysistest.end(env)

package_metadata_test = analysistest.make(_package_metadata_test_impl)

def _applicable_licenses_test_impl(ctx):
    env = analysistest.begin(ctx)
    info = analysistest.target_under_test(env)[PackageMetadataProbeInfo]
    asserts.equals(env, "sync_package_metadata.package-metadata.json", info.basename)
    return analysistest.end(env)

applicable_licenses_test = analysistest.make(_applicable_licenses_test_impl)

def _main_workspace_test_impl(ctx):
    env = analysistest.begin(ctx)
    info = analysistest.target_under_test(env)[PackageMetadataProbeInfo]
    asserts.equals(env, "", info.basename)
    return analysistest.end(env)

main_workspace_test = analysistest.make(_main_workspace_test_impl)

def module_info_test_suite():
    package_metadata(
        name = "cmp_package_metadata",
        purl = "pkg:golang/github.com/google/go-cmp@v0.6.0",
    )

    package_metadata_probe(
        name = "package_metadata_probe",
        module_label = "@com_github_google_go_cmp//cmp:go_default_library",
        package_metadata = [":cmp_package_metadata"],
        tags = ["manual"],
    )

    package_metadata_test(
        name = "package_metadata_test",
        target_under_test = ":package_metadata_probe",
    )

    package_metadata(
        name = "sync_package_metadata",
        purl = "pkg:golang/golang.org/x/sync@v0.8.0",
    )

    package_metadata_probe(
        name = "applicable_licenses_probe",
        module_label = "@org_golang_x_sync//errgroup:go_default_library",
        applicable_licenses = [":sync_package_metadata"],
        tags = ["manual"],
    )

    applicable_licenses_test(
        name = "applicable_licenses_test",
        target_under_test = ":applicable_licenses_probe",
    )

    package_metadata_probe(
        name = "main_workspace_probe",
        module_label = "//tests/core/starlark:main_workspace_probe",
        package_metadata = [":cmp_package_metadata"],
        tags = ["manual"],
    )

    main_workspace_test(
        name = "main_workspace_test",
        target_under_test = ":main_workspace_probe",
    )
