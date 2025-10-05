load("@platforms//host:constraints.bzl", "HOST_CONSTRAINTS")
load("@with_cfg.bzl", "with_cfg")
load("//proto:def.bzl", "go_proto_library")

def get_os():
    for c in HOST_CONSTRAINTS:
        if c.startswith("@platforms//os:"):
            return {
                "linux": "linux",
                "osx": "darwin",
                "windows": "windows",
            }[c.split(":")[-1]]

def get_cpu():
    for c in HOST_CONSTRAINTS:
        if c.startswith("@platforms//cpu:"):
            return {
                "x86_64": "amd64",
                "aarch64": "arm64",
            }[c.split(":")[-1]]

host_platform_go_proto_library, _host_platform_go_proto_library = (
    with_cfg(go_proto_library)
        .set("host_platform", Label("@io_bazel_rules_go//go/toolchain:{}_{}".format(get_os(), get_cpu())))
        .build()
)
