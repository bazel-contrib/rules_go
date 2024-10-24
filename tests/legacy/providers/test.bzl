load("//go:def.bzl", "GoInfo")

def _test_impl(ctx):
    pass

test_source = rule(
    implementation = _test_impl,
    attrs = {
        "srcs": attr.label(
            mandatory = True,
            providers = [GoInfo],
        ),
    },
)
