def _transition_sdk_impl(_, attr):
    return {"@my_rules_go//go/toolchain:sdk_version": str(attr.sdk_version)}

_transition_sdk = transition(
    implementation = _transition_sdk_impl,
    inputs = [],
    outputs = ["@my_rules_go//go/toolchain:sdk_version"],
)

def _sdk_transition_impl(ctx):
    executable = ctx.actions.declare_file(ctx.file.binary.basename)
    ctx.actions.symlink(
        output = executable,
        target_file = ctx.file.binary,
        is_executable = True,
    )
    return DefaultInfo(executable = executable)

sdk_transition_test = rule(
    _sdk_transition_impl,
    attrs = {
        "sdk_version": attr.string(mandatory = True),
        "binary": attr.label(
            allow_single_file = True,
            mandatory = True,
            cfg = "target",
        ),
        "_allowlist_function_transition": attr.label(
            default = "@bazel_tools//tools/allowlists/function_transition_allowlist",
        ),
    },
    cfg = _transition_sdk,
    test = True,
)
