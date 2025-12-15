def _root_symlinks_impl(ctx):
    return [
        DefaultInfo(
            runfiles = ctx.runfiles(
                root_symlinks = {
                    "link_" + f.basename: f
                    for f in ctx.files.data
                },
            ),
        ),
    ]

root_symlinks = rule(
    implementation = _root_symlinks_impl,
    attrs = {
        "data": attr.label_list(allow_files = True),
    },
)
