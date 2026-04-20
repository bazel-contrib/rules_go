load("@bazel_skylib//lib:unittest.bzl", "analysistest", "asserts")
load("@rules_license//rules:package_info.bzl", "package_info")
load("//go/private:context.bzl", "module_info_from_metadata")

ModuleInfoProbeInfo = provider()

def _module_info_probe_impl(ctx):
    info = module_info_from_metadata(
        Label(ctx.attr.module_label),
        getattr(ctx.attr, "package_metadata", ()),
        getattr(ctx.attr, "applicable_licenses", ()),
    )
    return [ModuleInfoProbeInfo(
        path = info.path,
        version = info.version,
    )]

module_info_probe = rule(
    implementation = _module_info_probe_impl,
    attrs = {
        "module_label": attr.string(mandatory = True),
        "package_metadata": attr.label_list(),
    },
)

def _package_metadata_module_info_test_impl(ctx):
    env = analysistest.begin(ctx)
    info = analysistest.target_under_test(env)[ModuleInfoProbeInfo]
    asserts.equals(env, "github.com/google/go-cmp", info.path)
    asserts.equals(env, "v0.6.0", info.version)
    return analysistest.end(env)

package_metadata_module_info_test = analysistest.make(_package_metadata_module_info_test_impl)

def _applicable_licenses_module_info_test_impl(ctx):
    env = analysistest.begin(ctx)
    info = analysistest.target_under_test(env)[ModuleInfoProbeInfo]
    asserts.equals(env, "golang.org/x/sync", info.path)
    asserts.equals(env, "v0.8.0", info.version)
    return analysistest.end(env)

applicable_licenses_module_info_test = analysistest.make(_applicable_licenses_module_info_test_impl)

def _main_workspace_module_info_test_impl(ctx):
    env = analysistest.begin(ctx)
    info = analysistest.target_under_test(env)[ModuleInfoProbeInfo]
    asserts.equals(env, "", info.path)
    asserts.equals(env, "", info.version)
    return analysistest.end(env)

main_workspace_module_info_test = analysistest.make(_main_workspace_module_info_test_impl)

def _versionless_module_info_test_impl(ctx):
    env = analysistest.begin(ctx)
    info = analysistest.target_under_test(env)[ModuleInfoProbeInfo]
    asserts.equals(env, "example.com/versionless", info.path)
    asserts.equals(env, "(devel)", info.version)
    return analysistest.end(env)

versionless_module_info_test = analysistest.make(_versionless_module_info_test_impl)

def module_info_test_suite():
    package_info(
        name = "cmp_package_info",
        package_name = "github.com/google/go-cmp",
        package_version = "0.6.0",
    )

    module_info_probe(
        name = "package_metadata_probe",
        module_label = "@com_github_google_go_cmp//cmp:go_default_library",
        package_metadata = [":cmp_package_info"],
        tags = ["manual"],
    )

    package_metadata_module_info_test(
        name = "package_metadata_module_info_test",
        target_under_test = ":package_metadata_probe",
    )

    package_info(
        name = "sync_package_info",
        package_name = "golang.org/x/sync",
        package_version = "0.8.0",
    )

    module_info_probe(
        name = "applicable_licenses_probe",
        module_label = "@org_golang_x_sync//errgroup:go_default_library",
        applicable_licenses = [":sync_package_info"],
        tags = ["manual"],
    )

    applicable_licenses_module_info_test(
        name = "applicable_licenses_module_info_test",
        target_under_test = ":applicable_licenses_probe",
    )

    module_info_probe(
        name = "main_workspace_probe",
        module_label = "//tests/core/starlark:main_workspace_probe",
        package_metadata = [":cmp_package_info"],
        tags = ["manual"],
    )

    main_workspace_module_info_test(
        name = "main_workspace_module_info_test",
        target_under_test = ":main_workspace_probe",
    )

    package_info(
        name = "versionless_package_info",
        package_name = "example.com/versionless",
        package_version = "",
    )

    module_info_probe(
        name = "versionless_probe",
        module_label = "@com_github_google_go_cmp//cmp:go_default_library",
        package_metadata = [":versionless_package_info"],
        tags = ["manual"],
    )

    versionless_module_info_test(
        name = "versionless_module_info_test",
        target_under_test = ":versionless_probe",
    )
