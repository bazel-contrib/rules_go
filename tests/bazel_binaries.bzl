"""Declares the native Bazel executable selected by the enclosing build."""

def _is_absolute(path):
    return (path.startswith("/") or path.startswith("\\")) or (len(path) > 2 and path[1] == ":" and path[2] in "/\\")

def _probe_bazel(ctx, path):
    # Bazelisk must not pass this probe by downloading/delegating to Bazel.
    return ctx.execute(
        [path, "--version"],
        environment = {"USE_BAZEL_VERSION": str(ctx.path("not-a-bazel-executable"))},
        quiet = True,
    )

def _selected_bazel_impl(ctx):
    env = ctx.os.environ
    override = env.get("GO_BAZEL_TEST_BINARY", "")
    selected = env.get("USE_BAZEL_VERSION", "")
    from_path = False
    binary = "bazel.exe" if ctx.os.name.lower().startswith("windows") else "bazel"
    if not ctx.attr.expected_version and not (override or _is_absolute(selected) or env.get("BAZEL_REAL")):
        fail("An unstamped enclosing Bazel requires an explicit native executable path via GO_BAZEL_TEST_BINARY or USE_BAZEL_VERSION")
    if override:
        if not _is_absolute(override):
            fail("GO_BAZEL_TEST_BINARY must be an absolute path to a native Bazel executable")
        path = ctx.path(override)
    elif _is_absolute(selected):
        # bazelci supplies the exact artifact for Bazel-under-test jobs.
        path = ctx.path(selected)
    elif env.get("BAZEL_REAL"):
        # Bazelisk sets this when invoking a workspace's tools/bazel wrapper.
        path = ctx.path(env["BAZEL_REAL"])
    else:
        # Bazelisk prepends the selected native executable's directory to PATH.
        # Resolve it now, not on the worker, where that path does not exist.
        path = ctx.which(binary)
        from_path = True
        if path == None:
            fail("Cannot locate the enclosing Bazel executable; set --repo_env=GO_BAZEL_TEST_BINARY=/absolute/path/to/bazel")

    if not path.exists:
        fail("The selected Bazel executable does not exist: {}".format(path))
    ctx.watch(path)
    path = path.realpath
    ctx.watch(path)

    result = _probe_bazel(ctx, path)
    if result.return_code != 0 and from_path:
        # Before Bazel 9, --action_env=PATH can hide the native directory
        # Bazelisk prepends. Ask the launcher for its resolved environment,
        # pinned to this build's exact version and without a workspace wrapper.
        # Never print this output: it contains the full client environment.
        version = ctx.attr.expected_version
        if "/" in selected:
            version = selected.rsplit("/", 1)[0] + "/" + version
        resolved = ctx.execute(
            [path, "--print_env"],
            environment = {
                "USE_BAZEL_VERSION": version,
                "BAZELISK_SKIP_WRAPPER": "1",
            },
            working_directory = str(ctx.workspace_root),
            quiet = True,
        )
        if resolved.return_code == 0:
            for line in resolved.stdout.splitlines():
                if line.lower().startswith("path="):
                    directory = line.partition("=")[2].split(";" if binary.endswith(".exe") else ":")[0]
                    if _is_absolute(directory):
                        path = ctx.path(directory + "/" + binary)
                        ctx.watch(path)
                        path = path.realpath
                        ctx.watch(path)
                        result = _probe_bazel(ctx, path)
                    break
    if result.return_code != 0:
        fail("Cannot execute native Bazel at {} (exit {}): {}; set --repo_env=GO_BAZEL_TEST_BINARY=/absolute/path/to/bazel".format(path, result.return_code, result.stderr[:4096]))
    version = result.stdout.strip()
    if ctx.attr.expected_version and version != "bazel " + ctx.attr.expected_version:
        fail("Selected Bazel reports '{}', but the enclosing build uses '{}'; provide the matching native executable".format(version, ctx.attr.expected_version))

    # Copy the bytes into the repository. An absolute host-path symlink would
    # not be a self-contained executable input on a remote worker.
    source = str(path).replace("\\", "/")
    if not source.startswith("/"):
        source = "/" + source
    source = source.replace("%", "%25").replace(" ", "%20").replace("#", "%23").replace("?", "%3F")
    ctx.download(
        url = "file://" + source,
        output = binary,
        executable = True,
    )
    ctx.file("version.txt", result.stdout)
    ctx.file("WORKSPACE", "")
    ctx.file("BUILD.bazel", """exports_files({files}, visibility = ["//visibility:public"])
alias(name = "binary", actual = {binary}, visibility = ["//visibility:public"])
""".format(files = repr([binary, "version.txt"]), binary = repr(binary)))

default_bazel_binary = repository_rule(
    implementation = _selected_bazel_impl,
    attrs = {"expected_version": attr.string(default = native.bazel_version)},
    environ = ["GO_BAZEL_TEST_BINARY", "USE_BAZEL_VERSION", "BAZEL_REAL", "PATH"],
    local = True,
    configure = True,
)

def _bazel_binaries_impl(ctx):
    default_bazel_binary(name = "rules_go_bazel_test")
    return ctx.extension_metadata(
        root_module_direct_deps = ["rules_go_bazel_test"],
        root_module_direct_dev_deps = [],
        # Reevaluate for each enclosing Bazel version rather than persisting
        # a version-dependent repository spec in a shared module lockfile.
        reproducible = True,
    )

bazel_binaries = module_extension(
    implementation = _bazel_binaries_impl,
)
