load("//proto:gogo.bzl", "gogo_special_proto")

def _gogo_impl(_ctx):
    gogo_special_proto(name = "gogo_special_proto")

gogo = module_extension(
    implementation = _gogo_impl,
)
