"""A rule that has a deps attribute but nothing to do with Go."""

def _non_go_rule_impl(_ctx):
    return []

non_go_rule = rule(
    implementation = _non_go_rule_impl,
    attrs = {
        "deps": attr.label_list(),
    },
)
