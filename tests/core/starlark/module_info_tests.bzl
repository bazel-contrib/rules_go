load("@bazel_skylib//lib:unittest.bzl", "analysistest", "asserts")
load("@package_metadata//rules:package_metadata.bzl", "package_metadata")
load("//go/private:context.bzl", "package_metadata_file_from_metadata")

PackageMetadataProbeInfo = provider()

def _package_metadata_probe_impl(ctx):
    metadata = package_metadata_file_from_metadata(
        getattr(ctx.attr, "package_metadata", ()),
        getattr(ctx.attr, "applicable_licenses", ()),
    )
    return [PackageMetadataProbeInfo(
        basename = metadata.basename if metadata else "",
    )]

package_metadata_probe = rule(
    implementation = _package_metadata_probe_impl,
)

def _package_metadata_test_impl(ctx):
    env = analysistest.begin(ctx)
    metadata = package_metadata_file_from_metadata(
        package_metadata = [analysistest.target_under_test(env)],
    )
    asserts.equals(env, "cmp_package_metadata.package-metadata.json", metadata.basename)
    return analysistest.end(env)

package_metadata_test = analysistest.make(_package_metadata_test_impl)

def _applicable_licenses_test_impl(ctx):
    env = analysistest.begin(ctx)
    info = analysistest.target_under_test(env)[PackageMetadataProbeInfo]
    asserts.equals(env, "sync_package_metadata.package-metadata.json", info.basename)
    return analysistest.end(env)

applicable_licenses_test = analysistest.make(_applicable_licenses_test_impl)

def module_info_test_suite():
    package_metadata(
        name = "cmp_package_metadata",
        purl = "pkg:golang/github.com/google/go-cmp@v0.6.0",
    )

    package_metadata_test(
        name = "package_metadata_test",
        target_under_test = ":cmp_package_metadata",
    )

    package_metadata(
        name = "sync_package_metadata",
        purl = "pkg:golang/golang.org/x/sync@v0.8.0",
    )

    package_metadata_probe(
        name = "applicable_licenses_probe",
        applicable_licenses = [":sync_package_metadata"],
        tags = ["manual"],
    )

    applicable_licenses_test(
        name = "applicable_licenses_test",
        target_under_test = ":applicable_licenses_probe",
    )
