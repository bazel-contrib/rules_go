load(":remote.bzl", "test_chdir_remote")

def _test_chdir_impl(_ctx):
    test_chdir_remote()

test_chdir = module_extension(
    implementation = _test_chdir_impl,
)
