load("@bazel_skylib//lib:paths.bzl", "paths")
load("@io_bazel_rules_go//go:def.bzl", "go_context", "new_go_info")

def _go_generated_library_impl(ctx):
    src = ctx.actions.declare_file("generated.go")
    ctx.actions.write(src, "package {}\n".format(paths.basename(ctx.attr.importpath)))

    go = go_context(ctx)
    go_info = new_go_info(
        go,
        ctx.attr,
        generated_srcs = [src],
        coverage_instrumented = ctx.coverage_instrumented(),
    )

    archive = go.archive(go, go_info)

    return [
        go_info,
        archive,
        DefaultInfo(
            files = depset([archive.data.file]),
            runfiles = archive.runfiles,
        ),
        OutputGroupInfo(
            go_generated_srcs = [src],
        ),
    ]

go_generated_library = rule(
    implementation = _go_generated_library_impl,
    attrs = {
        "importpath": attr.string(),
        "_go_context_data": attr.label(default = "@io_bazel_rules_go//:go_context_data"),
    },
    toolchains = ["@io_bazel_rules_go//go:toolchain"],
)
