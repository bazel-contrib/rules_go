load("@bazel_skylib//lib:unittest.bzl", "analysistest", "asserts")
load("@com_google_protobuf//bazel:proto_library.bzl", "proto_library")
load("//proto:def.bzl", "go_proto_library")

# A proto_library in go_proto_library's deps instead of its protos should get
# an error that says so, not one about a missing importpath attribute.
def _proto_library_in_deps_test_impl(ctx):
    env = analysistest.begin(ctx)
    asserts.expect_failure(env, "is a proto_library, which go_proto_library expects in 'protos'")
    return analysistest.end(env)

proto_library_in_deps_test = analysistest.make(
    _proto_library_in_deps_test_impl,
    expect_failure = True,
)

def proto_test_suite():
    proto_library(
        name = "misplaced_proto",
        tags = ["manual"],
    )

    go_proto_library(
        name = "proto_library_in_deps",
        importpath = "example.com/misplaced",
        deps = [":misplaced_proto"],
        tags = ["manual"],
    )

    proto_library_in_deps_test(
        name = "proto_library_in_deps_test",
        target_under_test = ":proto_library_in_deps",
    )
