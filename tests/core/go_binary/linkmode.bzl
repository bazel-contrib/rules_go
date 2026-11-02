load("//go/private/tools:copy_cmd.bzl", "copy_cmd")

_LINKMODE_SETTING = "//go/config:linkmode"

def _linkmode_pie_transition_impl(settings, attr):
    return {
        _LINKMODE_SETTING: "pie",
    }

_linkmode_pie_transition = transition(
    implementation = _linkmode_pie_transition_impl,
    inputs = [_LINKMODE_SETTING],
    outputs = [_LINKMODE_SETTING],
)

def _is_windows(ctx):
    return ctx.configuration.host_path_separator == ";"

def _linkmode_pie_wrapper(ctx):
    in_binary = ctx.attr.target[0][DefaultInfo].files.to_list()[0]
    out_binary = ctx.actions.declare_file(ctx.attr.name)

    # On windows symlinks are not reliable when using remote cache so we copy the binary instead.
    # See https://github.com/bazelbuild/bazel/issues/21747
    if _is_windows(ctx):
        copy_cmd(ctx, in_binary, out_binary)
    else:
        ctx.actions.symlink(output = out_binary, target_file = in_binary)
    return [
        DefaultInfo(
            files = depset([out_binary]),
        ),
    ]

linkmode_pie_wrapper = rule(
    implementation = _linkmode_pie_wrapper,
    doc = """Provides the (only) file produced by target, but after transitioning the linkmode setting to PIE.""",
    attrs = {
        "target": attr.label(
            cfg = _linkmode_pie_transition,
        ),
        "_allowlist_function_transition": attr.label(
            default = "@bazel_tools//tools/allowlists/function_transition_allowlist",
        ),
    },
)
