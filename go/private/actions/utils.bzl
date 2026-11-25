load(
    "@bazel_skylib//lib:shell.bzl",
    "shell",
)
load("//go/private:common.bzl", "SUPPORTS_PATH_MAPPING_REQUIREMENT")

def quote_opts(opts):
    return " ".join([shell.quote(opt) if " " in opt else opt for opt in opts])

def path_mapping_action_settings(go, cgo = False):
    """Returns the environment and execution requirements for a Go action."""
    if cgo or "local" in go._ctx.attr.tags:
        return go.env, {}
    return go.env_for_path_mapping, SUPPORTS_PATH_MAPPING_REQUIREMENT
