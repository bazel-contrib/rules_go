"""Module extension for repositories only used by CI tasks."""

load("@bazel_ci_rules//:rbe_repo.bzl", "rbe_preconfig")

def _bazelci_impl(_ctx):
    # Creates the default toolchain config the RBE task is configured to use
    # via --crosstool_top, --extra_toolchains and --host_platform.
    rbe_preconfig(
        name = "buildkite_config",
        toolchain = "ubuntu2204",
    )

bazelci = module_extension(
    implementation = _bazelci_impl,
)
