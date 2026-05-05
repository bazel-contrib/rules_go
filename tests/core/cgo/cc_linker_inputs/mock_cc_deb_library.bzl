def _mock_cc_deb_library_impl(ctx):
    expanded_linkopts = [
        ctx.expand_make_variables("linkopts", opt, {})
        for opt in ctx.attr.linkopts
    ]
    linker_input = cc_common.create_linker_input(
        owner = ctx.label,
        user_link_flags = depset(expanded_linkopts),
        additional_inputs = depset(ctx.files.additional_linker_inputs),
    )
    own_cc_info = CcInfo(linking_context = cc_common.create_linking_context(
        linker_inputs = depset([linker_input]),
    ))
    dep_cc_infos = [dep[CcInfo] for dep in ctx.attr.deps if CcInfo in dep]
    return [cc_common.merge_cc_infos(
        direct_cc_infos = [own_cc_info],
        cc_infos = dep_cc_infos,
    )]

mock_cc_deb_library = rule(
    implementation = _mock_cc_deb_library_impl,
    attrs = {
        "deps": attr.label_list(providers = [CcInfo]),
        "linkopts": attr.string_list(),
        "additional_linker_inputs": attr.label_list(allow_files = True),
    },
)
