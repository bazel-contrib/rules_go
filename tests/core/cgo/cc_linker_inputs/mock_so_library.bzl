def _mock_so_library_impl(ctx):
    cc_toolchain = ctx.toolchains["@bazel_tools//tools/cpp:toolchain_type"].cc
    feature_configuration = cc_common.configure_features(
        ctx = ctx,
        cc_toolchain = cc_toolchain,
    )
    libs = []
    for dyn_lib in ctx.files.dynamic_libs:
        ifso = ctx.actions.declare_file(dyn_lib.basename + ".ifso")
        ctx.actions.write(ifso, "/* empty interface library */")
        lib = cc_common.create_library_to_link(
            actions = ctx.actions,
            cc_toolchain = cc_toolchain,
            feature_configuration = feature_configuration,
            interface_library = ifso,
            dynamic_library = dyn_lib,
        )
        libs.append(lib)
    linker_input = cc_common.create_linker_input(
        owner = ctx.label,
        libraries = depset(libs),
    )
    return [CcInfo(linking_context = cc_common.create_linking_context(
        linker_inputs = depset([linker_input]),
    ))]

mock_so_library = rule(
    implementation = _mock_so_library_impl,
    attrs = {
        "dynamic_libs": attr.label_list(allow_files = True),
    },
    toolchains = ["@bazel_tools//tools/cpp:toolchain_type"],
    fragments = ["cpp"],
)
