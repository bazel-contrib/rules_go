# bazel_features when used from a WORKSPACE rather than bzlmod context requires a two-step set-up (loading bazel_features, then calling a function from inside it).
# rules_go only has one-step set-up, and it would be a breaking change to require a second step.
# Accordingly, we supply a polyfill implementation of bazel_features which is only used when using rules_go from a WORKSPACE file,
# to avoid complicating code in rules_go itself.
# We just implement the checks we've seen we actually need, and hope to delete this completely when we are in a pure-bzlmod world.

_POLYFILL_BAZEL_FEATURES = """bazel_features = struct(
  external_deps = struct(
    # WORKSPACE users have no use for bazel mod tidy.
    bazel_mod_tidy = False,
  ),
)
"""

def _polyfill_bazel_features_impl(rctx):
    rctx.file("BUILD.bazel", """
load("@bazel_skylib//:bzl_library.bzl", "bzl_library")
bzl_library(
    name = "features",
    srcs = ["features.bzl"],
    visibility = ["//visibility:public"],
)
exports_files(["features.bzl"])
""")
    rctx.file("features.bzl", _POLYFILL_BAZEL_FEATURES)

polyfill_bazel_features = repository_rule(
    implementation = _polyfill_bazel_features_impl,
    # Force reruns on server restarts to keep native.bazel_version up-to-date.
    local = True,
)
