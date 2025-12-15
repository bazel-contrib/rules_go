load("@bazel_skylib//lib:unittest.bzl", "asserts", "unittest")
load("//go/private:common.bzl", "count_group_matches", "has_shared_lib_extension")

def _versioned_shared_libraries_test(ctx):
    env = unittest.begin(ctx)

    # See //src/test/java/com/google/devtools/build/lib/rules/cpp:CppFileTypesTest.java
    # for the corresponding native C++ rules tests.
    asserts.true(env, has_shared_lib_extension("somelibrary.so"))
    asserts.true(env, has_shared_lib_extension("somelibrary.so.2"))
    asserts.true(env, has_shared_lib_extension("somelibrary.so.20"))
    asserts.true(env, has_shared_lib_extension("somelibrary.so.20.2"))
    asserts.true(env, has_shared_lib_extension("a/somelibrary.so.2"))
    asserts.true(env, has_shared_lib_extension("somelibrary✅.so.2"))
    asserts.true(env, has_shared_lib_extension("somelibrary✅.so.2.1"))
    asserts.false(env, has_shared_lib_extension("somelibrary.so.e"))
    asserts.false(env, has_shared_lib_extension("xx.1"))
    asserts.true(env, has_shared_lib_extension("somelibrary.so.2e"))
    asserts.false(env, has_shared_lib_extension("somelibrary.so.e2"))
    asserts.false(env, has_shared_lib_extension("somelibrary.so.20.e2"))
    asserts.false(env, has_shared_lib_extension("somelibrary.a.2"))
    asserts.false(env, has_shared_lib_extension("somelibrary.a..2"))
    asserts.false(env, has_shared_lib_extension("somelibrary.so.2."))
    asserts.false(env, has_shared_lib_extension("somelibrary.so."))
    asserts.false(env, has_shared_lib_extension("somelibrary.so.2🚫"))
    asserts.false(env, has_shared_lib_extension("somelibrary.so.🚫2"))
    asserts.false(env, has_shared_lib_extension("somelibrary.so🚫.2.0"))
    asserts.false(env, has_shared_lib_extension("somelibrary.so.2$"))
    asserts.true(env, has_shared_lib_extension("somelibrary.so.1a_b2"))
    asserts.false(env, has_shared_lib_extension("libA.so.gen.empty.def"))
    asserts.false(env, has_shared_lib_extension("libA.so.if.exp"))
    asserts.false(env, has_shared_lib_extension("libA.so.if.lib"))
    asserts.true(env, has_shared_lib_extension("libaws-c-s3.so.0unstable"))

    return unittest.end(env)

versioned_shared_libraries_test = unittest.make(_versioned_shared_libraries_test)

def _count_group_matches_test(ctx):
    env = unittest.begin(ctx)

    asserts.equals(env, 1, count_group_matches("{foo_status}", "{foo_", "}"))
    asserts.equals(env, 1, count_group_matches("{foo_status} {status}", "{foo_", "}"))
    asserts.equals(env, 0, count_group_matches("{foo_status}", "{bar_", "}"))
    asserts.equals(env, 1, count_group_matches("{foo_status}", "{", "}"))
    asserts.equals(env, 2, count_group_matches("{foo} {bar}", "{", "}"))
    asserts.equals(env, 2, count_group_matches("{foo{bar} {baz}", "{", "}"))

    return unittest.end(env)

count_group_matches_test = unittest.make(_count_group_matches_test)

def common_test_suite():
    """Creates the test targets and test suite for common.bzl tests."""
    unittest.suite(
        "common_tests",
        versioned_shared_libraries_test,
        count_group_matches_test,
    )
