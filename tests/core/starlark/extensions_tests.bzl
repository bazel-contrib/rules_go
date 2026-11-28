load("@bazel_skylib//lib:unittest.bzl", "asserts", "unittest")
load("//go/private:extensions.bzl", "drop_shadowed_dependency_toolchains")

def _toolchain(sdk_repo, goos, goarch, root):
    return struct(
        root = root,
        goos = goos,
        goarch = goarch,
        sdk_repo = sdk_repo,
        sdk_type = "remote",
        sdk_version = "1.23.1",
    )

def _drop_shadowed_dependency_toolchains_test(ctx):
    env = unittest.begin(ctx)

    toolchains = [
        _toolchain("main_host", "", "", root = True),
        _toolchain("main_linux_arm64", "linux", "arm64", root = True),
        _toolchain("main_linux_arm64_other", "linux", "arm64", root = True),
        _toolchain("dep_host", "", "", root = False),
        _toolchain("dep_linux_amd64", "linux", "amd64", root = False),
        _toolchain("dep_linux_arm64", "linux", "arm64", root = False),
        _toolchain("dep_windows_amd64", "windows", "amd64", root = False),
    ]
    asserts.equals(
        env,
        ["main_host", "main_linux_arm64", "main_linux_arm64_other", "dep_windows_amd64"],
        [t.sdk_repo for t in drop_shadowed_dependency_toolchains(toolchains, "linux", "amd64")],
    )

    return unittest.end(env)

drop_shadowed_dependency_toolchains_test = unittest.make(_drop_shadowed_dependency_toolchains_test)

def extensions_test_suite():
    unittest.suite(
        "extensions_tests",
        drop_shadowed_dependency_toolchains_test,
    )
