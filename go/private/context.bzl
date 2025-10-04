# Copyright 2017 The Bazel Authors. All rights reserved.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#    http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

load(
    "@bazel_skylib//lib:structs.bzl",
    "structs",
)
load(
    "@bazel_skylib//rules:common_settings.bzl",
    "BuildSettingInfo",
)
load(
    "@bazel_tools//tools/build_defs/cc:action_names.bzl",
    "CPP_COMPILE_ACTION_NAME",
    "CPP_LINK_DYNAMIC_LIBRARY_ACTION_NAME",
    "CPP_LINK_EXECUTABLE_ACTION_NAME",
    "CPP_LINK_STATIC_LIBRARY_ACTION_NAME",
    "C_COMPILE_ACTION_NAME",
    "OBJCPP_COMPILE_ACTION_NAME",
    "OBJC_COMPILE_ACTION_NAME",
)
load(
    "@bazel_tools//tools/cpp:toolchain_utils.bzl",
    "find_cpp_toolchain",
)
load(
    "@io_bazel_rules_nogo//:scope.bzl",
    NOGO_EXCLUDES = "EXCLUDES",
    NOGO_INCLUDES = "INCLUDES",
)
load(
    "//go/platform:apple.bzl",
    "apple_ensure_options",
)
load(
    "//go/private/rules:transition.bzl",
    "non_request_nogo_transition",
    "request_nogo_transition",
)
load(
    ":common.bzl",
    "COVERAGE_OPTIONS_DENYLIST",
    "GO_TOOLCHAIN",
    "as_iterable",
)
load(
    ":mode.bzl",
    "LINKMODE_NORMAL",
    "installsuffix",
    "validate_mode",
)
load(
    ":providers.bzl",
    "CgoContextInfo",
    "EXPLICIT_PATH",
    "EXPORT_PATH",
    "GoArchive",
    "GoConfigInfo",
    "GoContextInfo",
    "GoInfo",
    "GoStdLib",
    "INFERRED_PATH",
    "get_archive",
    "get_source",
)

CPP_TOOLCHAIN_TYPE = Label("@bazel_tools//tools/cpp:toolchain_type")
CGO_ATTRS = {
    "_cc_toolchain": attr.label(default = "@bazel_tools//tools/cpp:optional_current_cc_toolchain"),
    "_xcode_config": attr.label(default = "@bazel_tools//tools/osx:current_xcode_config"),
}
CGO_TOOLCHAINS = [
    # In pure mode, a C++ toolchain isn't needed when transitioning.
    # But if we declare a mandatory toolchain dependency here, a cross-compiling C++ toolchain is required at toolchain resolution time.
    # So we make this toolchain dependency optional, so that it's only attempted to be looked up if it's actually needed.
    config_common.toolchain_type(CPP_TOOLCHAIN_TYPE, mandatory = False),
]
CGO_FRAGMENTS = ["apple", "cpp"]

# cgo requires a gcc/clang style compiler.
# We use a denylist instead of an allowlist:
# - Bazel's auto-detected toolchains used to set the compiler name to "compiler"
#   for gcc (fixed in 6.0.0), which defeats the purpose of an allowlist.
# - The compiler name field is free-form and user-defined, so we would have to
#   provide a way to override this list.
# TODO: Convert to a denylist once we can assume Bazel 6.0.0 or later and have a
#       way for users to extend the list.
_UNSUPPORTED_C_COMPILERS = {
    "msvc-cl": None,
    "clang-cl": None,
}

_COMPILER_OPTIONS_DENYLIST = dict({
    # cgo parses the error messages from the compiler.  It can't handle colors.
    # Ignore both variants of the diagnostics color flag.
    "-fcolor-diagnostics": None,
    "-fdiagnostics-color": None,

    # cgo also wants to see all the errors when it is testing the compiler.
    # fmax-errors limits that and causes build failures.
    "-fmax-errors=": None,
    "-Wall": None,

    # Symbols are needed by Go, so keep them
    "-g0": None,

    # Don't compile generated cgo code with coverage. If we do an internal
    # link, we may have undefined references to coverage functions.
    "--coverage": None,
    "-ftest-coverage": None,
    "-fprofile-arcs": None,
    "-fprofile-instr-generate": None,
    "-fcoverage-mapping": None,
}, **COVERAGE_OPTIONS_DENYLIST)

_LINKER_OPTIONS_DENYLIST = {
    "-Wl,--gc-sections": None,
    "-pie": None,  # See https://github.com/bazelbuild/rules_go/issues/3691.
}

_UNSUPPORTED_FEATURES = [
    # These toolchain features require special rule support and will thus break
    # with CGo.
    # Taken from https://github.com/bazelbuild/rules_rust/blob/521e649ff44e9711fe3c45b0ec1e792f7e1d361e/rust/private/utils.bzl#L20.
    "thin_lto",
    "module_maps",
    "use_header_modules",
    "fdo_instrument",
    "fdo_optimize",
    # This is a nonspecific unsupported feature which allows the authors of C++
    # toolchain to apply separate flags when compiling Go code.
    "rules_go_unsupported_feature",
]

def _match_option(option, pattern):
    if pattern.endswith("="):
        return option.startswith(pattern)
    else:
        return option == pattern

def _filter_options(options, denylist):
    return [
        option
        for option in options
        if not any([_match_option(option, pattern) for pattern in denylist])
    ]

def _child_name(go, path, ext, name):
    if not name:
        name = go.label.name
        if path or not ext:
            # The '_' avoids collisions with another file matching the label name.
            # For example, hello and hello/testmain.go.
            name += "_"
    if path:
        name += "/" + path
    if ext:
        name += ext
    return name

def _declare_file(go, path = "", ext = "", name = ""):
    return go.actions.declare_file(_child_name(go, path, ext, name))

def _declare_directory(go, path = "", ext = "", name = ""):
    return go.actions.declare_directory(_child_name(go, path, ext, name))

def _dirname(file):
    return file.dirname

def _builder_args(go, command = None):
    args = go.actions.args()
    args.use_param_file("-param=%s")
    if command:
        args.add(command)
    sdk_root_file = go.sdk.root_file
    args.add("-sdk", sdk_root_file.dirname)

    # Path mapping can't map the values of environment variables, so we need to pass GOROOT to the
    # action via an argument instead.
    if go.stdlib:
        goroot_file = go.stdlib.root_file
    else:
        goroot_file = sdk_root_file

    # Use a file rather than goroot as the latter is just a string and thus
    # not subject to path mapping.
    args.add_all("-goroot", [goroot_file], map_each = _dirname, expand_directories = False)

    mode = go.mode
    args.add("-installsuffix", installsuffix(mode))
    args.add_joined("-tags", mode.tags, join_with = ",")
    return args

def _tool_args(go):
    args = go.actions.args()
    args.use_param_file("-param=%s")
    return args

def _merge_embed(source, embed):
    s = get_source(embed)
    source["srcs"] = s.srcs + source["srcs"]
    source["embedsrcs"] = source["embedsrcs"] + s.embedsrcs
    source["cover"] = depset(transitive = [source["cover"], s.cover])
    source["deps"] = source["deps"] + s.deps
    source["x_defs"].update(s.x_defs)
    source["gc_goopts"] = source["gc_goopts"] + s.gc_goopts
    source["runfiles"] = source["runfiles"].merge(s.runfiles)

    if s.cgo:
        if source["cgo"]:
            fail("multiple libraries with cgo enabled")
        source["cgo"] = s.cgo
        source["cdeps"] = s.cdeps
        source["cppopts"] = s.cppopts
        source["copts"] = s.copts
        source["cxxopts"] = s.cxxopts
        source["clinkopts"] = s.clinkopts

def _dedup_archives(archives):
    """Returns a list of archives without duplicate import paths.

    Earlier archives take precedence over later targets. This is intended to
    allow an embedding library to override the dependencies of its
    embedded libraries.

    Args:
      archives: an iterable of GoArchives.
    """
    deduped_archives = []
    importpaths = {}
    for arc in archives:
        importpath = arc.data.importpath
        if importpath in importpaths:
            continue
        importpaths[importpath] = None
        deduped_archives.append(arc)
    return deduped_archives

def _deprecated_new_library(go, name = None, importpath = None, resolver = None, importable = True, testfilter = None, is_main = False, **kwargs):
    if not importpath:
        importpath = go.importpath
        importmap = go.importmap
    else:
        importmap = importpath
    pathtype = go.pathtype
    if not importable and pathtype == EXPLICIT_PATH:
        pathtype = EXPORT_PATH

    return struct(
        name = go.label.name if not name else name,
        label = go.label,
        importpath = importpath,
        importmap = importmap,
        importpath_aliases = go.importpath_aliases,
        pathtype = pathtype,
        resolve = resolver,
        testfilter = testfilter,
        is_main = is_main,
        **kwargs
    )

def _deprecated_library_to_source(go, attr, library, coverage_instrumented, verify_resolver_deps = True):
    return new_go_info(
        go,
        attr,
        name = library.name,
        importpath = library.importpath,
        resolver = library.resolve,
        testfilter = library.testfilter,
        is_main = library.is_main,
        coverage_instrumented = coverage_instrumented,
        generated_srcs = getattr(library, "srcs", []),
        pathtype = library.pathtype,
        verify_resolver_deps = verify_resolver_deps,
    )

def new_go_info(
        go,
        attr,
        name = None,
        importpath = None,
        resolver = None,
        importable = True,
        testfilter = None,
        is_main = False,
        coverage_instrumented = None,
        generated_srcs = [],
        pathtype = None,
        verify_resolver_deps = False):
    if not importpath:
        importpath = go.importpath
        importmap = go.importmap
    else:
        importmap = importpath
    if not pathtype:
        pathtype = go.pathtype
    if not importable and pathtype == EXPLICIT_PATH:
        pathtype = EXPORT_PATH

    if coverage_instrumented == None:
        coverage_instrumented = go.coverage_instrumented

    #TODO: stop collapsing a depset in this line...
    attr_srcs = [f for t in getattr(attr, "srcs", []) for f in as_iterable(t.files)]
    srcs = attr_srcs + generated_srcs
    embedsrcs = [f for t in getattr(attr, "embedsrcs", []) for f in as_iterable(t.files)]
    deps = [get_archive(dep) for dep in getattr(attr, "deps", [])]
    data = getattr(attr, "data", [])

    go_info = {
        "name": go.label.name if not name else name,
        "label": go.label,
        "importpath": importpath,
        "importmap": importmap,
        "importpath_aliases": tuple(getattr(attr, "importpath_aliases", ())),
        "pathtype": pathtype,
        "testfilter": testfilter,
        "is_main": is_main,
        "mode": go.mode,
        "srcs": srcs,
        "embedsrcs": embedsrcs,
        "cover": depset(attr_srcs) if coverage_instrumented else depset(),
        "x_defs": {},
        "deps": deps,
        "gc_goopts": _expand_opts(go, "gc_goopts", getattr(attr, "gc_goopts", [])),
        "runfiles": _collect_runfiles(go, data, deps),
        "cgo": getattr(attr, "cgo", False),
        "cdeps": getattr(attr, "cdeps", []),
        "cppopts": _expand_opts(go, "cppopts", getattr(attr, "cppopts", [])),
        "copts": _expand_opts(go, "copts", getattr(attr, "copts", [])),
        "cxxopts": _expand_opts(go, "cxxopts", getattr(attr, "cxxopts", [])),
        "clinkopts": _expand_opts(go, "clinkopts", getattr(attr, "clinkopts", [])),
        "pgoprofile": getattr(attr, "pgoprofile", None),
    }

    for e in getattr(attr, "embed", []):
        _merge_embed(go_info, e)

    go_info["deps"] = _dedup_archives(go_info["deps"])

    x_defs = go_info["x_defs"]

    for k, v in getattr(attr, "x_defs", {}).items():
        if "$" in v:
            v = go._ctx.expand_location(v, data)
        if "." not in k:
            k = "%s.%s" % (importmap, k)
        x_defs[k] = v
    go_info["x_defs"] = x_defs

    if not go_info["cgo"]:
        for k in ("cdeps", "cppopts", "copts", "cxxopts", "clinkopts"):
            if getattr(attr, k, None):
                fail(k + " set without cgo = True")
        for f in go_info["srcs"]:
            # This check won't report directory sources that contain C/C++
            # sources. compilepkg will catch these instead.
            if f.extension in ("c", "cc", "cxx", "cpp", "hh", "hpp", "hxx"):
                fail("source {} has C/C++ extension, but cgo was not enabled (set 'cgo = True')".format(f.path))

    if resolver:
        resolver(go, attr, go_info, _merge_embed)

        # TODO(zbarsky): Remove this once downstream has a chance to migrate.
        if verify_resolver_deps:
            for dep in go_info["deps"]:
                if type(dep) == "Target":
                    print('Detected Targets in `source["deps"]` as a result of _resolver: ' +
                          "{}, from target {}. ".format(resolver, str(go.label)) +
                          "Please pass a list of `GoArchive`s instead, for examples `deps = [deps[GoArchive] for dep in deps]`. " +
                          "This will be an error in the future.")
                    go_info["deps"] = [get_archive(dep) for dep in go_info["deps"]]
                    break

    return GoInfo(**go_info)

def _collect_runfiles(go, data, deps):
    """Builds a set of runfiles from the deps and data attributes.

    srcs and their runfiles are not included."""
    files = depset(transitive = [t[DefaultInfo].files for t in data])
    return go._ctx.runfiles(transitive_files = files).merge_all(
        [t[DefaultInfo].data_runfiles for t in data] +
        [get_source(t).runfiles for t in deps],
    )

def _infer_importpath(ctx, embeds, importpath, importmap):
    VENDOR_PREFIX = "/vendor/"

    # Check if paths were explicitly set, either in this rule or in an
    # embedded rule.
    embed_importpath = ""
    embed_importmap = ""
    for embed in embeds:
        lib = embed[GoInfo]
        if lib.pathtype == EXPLICIT_PATH:
            embed_importpath = lib.importpath
            embed_importmap = lib.importmap
            break

    importpath = importpath or embed_importpath
    importmap = importmap or embed_importmap or importpath
    if importpath:
        return importpath, importmap, EXPLICIT_PATH

    # Guess an import path based on the directory structure
    # This should only be executed for binaries, since Gazelle generates importpath for libraries.
    importpath = ctx.label.package
    if not importpath.endswith(ctx.label.name):
        importpath += "/" + ctx.label.name
    if importpath.rfind(VENDOR_PREFIX) != -1:
        importpath = importpath[len(VENDOR_PREFIX) + importpath.rfind(VENDOR_PREFIX):]
    if importpath.startswith("/"):
        importpath = importpath[1:]
    return importpath, importpath, INFERRED_PATH

def matches_scope(label, scope):
    if scope == "all":
        return True
    if scope.repo_name != label.repo_name:
        return False
    if scope.name == "__pkg__":
        return scope.package == label.package
    if scope.name == "__subpackages__":
        if not scope.package:
            return True
        return scope.package == label.package or label.package.startswith(scope.package + "/")
    fail("invalid scope '%s'" % scope.name)

def _matches_scopes(label, scopes):
    for scope in scopes:
        if matches_scope(label, scope):
            return True
    return False

def validate_nogo(go):
    """Whether nogo should be run as a validation action rather than just to generate fact files for the current
    target."""
    label = go.label
    return _matches_scopes(label, NOGO_INCLUDES) and not _matches_scopes(label, NOGO_EXCLUDES)

default_go_config_info = GoConfigInfo(
    static = False,
    race = False,
    msan = False,
    pure = False,
    strip = False,
    debug = False,
    linkmode = LINKMODE_NORMAL,
    gc_linkopts = [],
    tags = [],
    stamp = False,
    cover_format = None,
    gc_goopts = [],
    amd64 = None,
    arm = None,
    pgoprofile = None,
    export_stdlib = False,
)

def go_context(
        ctx,
        attr = None,
        include_deprecated_properties = True,
        importpath = None,
        importmap = None,
        embed = None,
        importpath_aliases = None,
        go_context_data = None,
        goos = "auto",
        goarch = "auto"):
    """Returns an API used to build Go code.

    See /go/toolchains.rst#go-context
    """
    if not attr:
        attr = ctx.attr
    toolchain = ctx.toolchains[GO_TOOLCHAIN]
    cgo_context_info = None
    go_context_info = None
    go_config_info = None
    stdlib = None

    if go_context_data == None:
        if hasattr(attr, "_go_context_data"):
            go_context_data = attr._go_context_data
            go_config_info = go_context_data[GoConfigInfo]
            stdlib = go_context_data[GoStdLib]
            go_context_info = go_context_data[GoContextInfo]
        if hasattr(attr, "_go_config"):
            go_config_info = attr._go_config[GoConfigInfo]
        if hasattr(attr, "_stdlib"):
            stdlib = attr._stdlib[GoStdLib]
    else:
        go_config_info = go_context_data[GoConfigInfo]
        stdlib = go_context_data[GoStdLib]
        go_context_info = go_context_data[GoContextInfo]

    if getattr(attr, "_cc_toolchain", None) and CPP_TOOLCHAIN_TYPE in ctx.toolchains:
        cgo_context_info = cgo_context_data_impl(ctx)
    elif go_context_data and CgoContextInfo in go_context_data:
        cgo_context_info = go_context_data[CgoContextInfo]
    elif getattr(attr, "_cgo_context_data", None) and CgoContextInfo in attr._cgo_context_data:
        cgo_context_info = attr._cgo_context_data[CgoContextInfo]
    elif getattr(attr, "cgo_context_data", None) and CgoContextInfo in attr.cgo_context_data:
        cgo_context_info = attr.cgo_context_data[CgoContextInfo]

    if goos == "auto" and goarch == "auto" and cgo_context_info and (go_config_info == None or not go_config_info.pure):
        # Fast-path to reuse the GoConfigInfo as-is
        mode = go_config_info or default_go_config_info
    else:
        if go_config_info == None:
            go_config_info = default_go_config_info
        mode_kwargs = structs.to_dict(go_config_info)
        mode_kwargs["goos"] = toolchain.default_goos if goos == "auto" else goos
        mode_kwargs["goarch"] = toolchain.default_goarch if goarch == "auto" else goarch
        if not cgo_context_info:
            if getattr(ctx.attr, "pure", None) == "off":
                fail("{} has pure explicitly set to off, but no C++ toolchain could be found for its platform".format(ctx.label))
            mode_kwargs["pure"] = True
        mode = GoConfigInfo(**mode_kwargs)
        validate_mode(mode)

    if stdlib:
        goroot = stdlib.root_file.dirname
    else:
        goroot = toolchain.sdk.root_file.dirname

    env = {
        "GOARCH": mode.goarch,
        "GOOS": mode.goos,
        "GOEXPERIMENT": toolchain.sdk.experiments,
        "GOROOT": goroot,
        "GOROOT_FINAL": "GOROOT",
        "CGO_ENABLED": "0" if mode.pure else "1",

        # If we use --action_env=GOPATH, or in other cases where environment
        # variables are passed through to this builder, the SDK build will try
        # to write to that GOPATH (e.g. for x/net/nettest). This will fail if
        # the GOPATH is on a read-only mount, and is generally a bad idea.
        # Explicitly clear this environment variable to ensure that doesn't
        # happen. See #2291 for more information.
        "GOPATH": "",

        # Since v1.21.0, set GOTOOLCHAIN to "local" to use the current toolchain
        # and avoid downloading other toolchains.
        #
        # See https://go.dev/doc/toolchain for more info.
        "GOTOOLCHAIN": "local",

        # NOTE(#4049): Since Go 1.23.0, os.Readlink (and consequently
        # filepath.EvalSymlinks) stopped treating Windows mount points and
        # reparse points (junctions) as symbolic links. Bazel uses junctions
        # when constructing exec roots, and we use filepath.EvalSymlinks in
        # GoStdlib, so this broke us. Setting GODEBUG=winsymlink=0 restores
        # the old behavior.
        "GODEBUG": "winsymlink=0",
    }

    # The level of support is determined by the platform constraints in
    # //go/constraints/amd64.
    # See https://go.dev/wiki/MinimumRequirements#amd64
    if mode.amd64:
        env["GOAMD64"] = mode.amd64

    # Similarly, set GOARM based on platform constraints in //go/constraints/arm.
    # See https://go.dev/wiki/MinimumRequirements#arm
    if mode.arm:
        env["GOARM"] = mode.arm

    if cgo_context_info:
        env.update(cgo_context_info.env)
        cc_toolchain_files = cgo_context_info.cc_toolchain_files
        cgo_tools = cgo_context_info.cgo_tools
    else:
        cc_toolchain_files = depset()
        cgo_tools = None

    if importpath == None:
        importpath = getattr(attr, "importpath", "")
    if ":" in importpath:
        fail("import path '%s' contains invalid character :" % importpath)

    if importmap == None:
        importmap = getattr(attr, "importmap", "")
    if ":" in importmap:
        fail("import path '%s' contains invalid character :" % importmap)

    if importpath_aliases == None:
        importpath_aliases = getattr(attr, "importpath_aliases", ())
    for p in importpath_aliases:
        if ":" in p:
            fail("import path '%s' contains invalid character :" % p)

    if embed == None:
        embed = getattr(attr, "embed", [])

    importpath, importmap, pathtype = _infer_importpath(ctx, embed, importpath, importmap)

    if include_deprecated_properties:
        deprecated_properties = {
            "root": goroot,
            "go": toolchain.sdk.go,
            "sdk_root": toolchain.sdk.root_file,
            "sdk_tools": toolchain.sdk.tools,
            "package_list": toolchain.sdk.package_list,
            "tags": mode.tags,
            "stamp": mode.stamp,
            "cover_format": mode.cover_format,
            "pgoprofile": mode.pgoprofile,
        }
    else:
        deprecated_properties = {}

    return struct(
        # Fields
        toolchain = toolchain,
        sdk = toolchain.sdk,
        mode = mode,
        stdlib = stdlib,
        actions = ctx.actions,
        cc_toolchain_files = cc_toolchain_files,
        importpath = importpath,
        importmap = importmap,
        importpath_aliases = importpath_aliases,
        pathtype = pathtype,
        cgo_tools = cgo_tools,
        nogo = go_context_info.nogo if go_context_info else None,
        coverdata = go_context_info.coverdata if go_context_info else None,
        coverage_enabled = ctx.configuration.coverage_enabled,
        coverage_instrumented = ctx.coverage_instrumented(),
        export_stdlib = go_config_info.export_stdlib,
        env = env,
        # Path mapping can't map the values of environment variables, so we pass GOROOT to the action
        # via an argument instead in builder_args. We need to drop it from the environment to get cache
        # hits across different configurations since the stdlib path typically contains a Bazel
        # configuration segment.
        env_for_path_mapping = {k: v for k, v in env.items() if k != "GOROOT"},
        label = ctx.label,

        # Action generators
        archive = toolchain.actions.archive,
        binary = toolchain.actions.binary,
        link = toolchain.actions.link,

        # Helpers
        builder_args = _builder_args,
        tool_args = _tool_args,
        new_library = _deprecated_new_library,
        library_to_source = _deprecated_library_to_source,
        declare_file = _declare_file,
        declare_directory = _declare_directory,

        # Private
        # TODO: All uses of this should be removed
        _ctx = ctx,

        # Deprecated
        **deprecated_properties
    )

def _go_context_data_impl(ctx):
    if "race" in ctx.features:
        print("WARNING: --features=race is no longer supported. Use --@io_bazel_rules_go//go/config:race instead.")
    if "msan" in ctx.features:
        print("WARNING: --features=msan is no longer supported. Use --@io_bazel_rules_go//go/config:msan instead.")
    providers = [
        GoContextInfo(
            coverdata = ctx.attr.coverdata[0][GoArchive],
            nogo = ctx.attr.nogo[DefaultInfo].files_to_run,
        ),
        ctx.attr.stdlib[GoStdLib],
        ctx.attr.go_config[GoConfigInfo],
    ]
    if ctx.attr.cgo_context_data and CgoContextInfo in ctx.attr.cgo_context_data:
        providers.append(ctx.attr.cgo_context_data[CgoContextInfo])
    return providers

go_context_data = rule(
    _go_context_data_impl,
    attrs = {
        "cgo_context_data": attr.label(),
        "coverdata": attr.label(
            mandatory = True,
            cfg = non_request_nogo_transition,
            providers = [GoArchive],
        ),
        "go_config": attr.label(
            mandatory = True,
            providers = [GoConfigInfo],
        ),
        "nogo": attr.label(
            mandatory = True,
            cfg = "exec",
        ),
        "stdlib": attr.label(
            mandatory = True,
            providers = [GoStdLib],
        ),
        "_allowlist_function_transition": attr.label(
            default = "@bazel_tools//tools/allowlists/function_transition_allowlist",
        ),
    },
    doc = """go_context_data gathers information about the build configuration.
    It is a common dependency of all Go targets.""",
    toolchains = [GO_TOOLCHAIN],
    cfg = request_nogo_transition,
)

def cgo_context_data_impl(ctx):
    # TODO(jayconrod): find a way to get a list of files that comprise the
    # toolchain (to be inputs into actions that need it).
    # ctx.files._cc_toolchain won't work when cc toolchain resolution
    # is switched on.
    cc_toolchain = find_cpp_toolchain(ctx, mandatory = False)
    if not cc_toolchain or cc_toolchain.compiler in _UNSUPPORTED_C_COMPILERS:
        return None

    feature_configuration = cc_common.configure_features(
        ctx = ctx,
        cc_toolchain = cc_toolchain,
        requested_features = ctx.features,
        unsupported_features = ctx.disabled_features + _UNSUPPORTED_FEATURES,
    )

    # TODO(jayconrod): keep the environment separate for different actions.
    env = {}

    c_compile_variables = cc_common.create_compile_variables(
        feature_configuration = feature_configuration,
        cc_toolchain = cc_toolchain,
    )
    c_compiler_path = cc_common.get_tool_for_action(
        feature_configuration = feature_configuration,
        action_name = C_COMPILE_ACTION_NAME,
    )
    c_compile_options = _filter_options(
        cc_common.get_memory_inefficient_command_line(
            feature_configuration = feature_configuration,
            action_name = C_COMPILE_ACTION_NAME,
            variables = c_compile_variables,
        ) + ctx.fragments.cpp.copts + ctx.fragments.cpp.conlyopts,
        _COMPILER_OPTIONS_DENYLIST,
    )
    env.update(cc_common.get_environment_variables(
        feature_configuration = feature_configuration,
        action_name = C_COMPILE_ACTION_NAME,
        variables = c_compile_variables,
    ))

    cxx_compile_variables = cc_common.create_compile_variables(
        feature_configuration = feature_configuration,
        cc_toolchain = cc_toolchain,
    )
    cxx_compile_options = _filter_options(
        cc_common.get_memory_inefficient_command_line(
            feature_configuration = feature_configuration,
            action_name = CPP_COMPILE_ACTION_NAME,
            variables = cxx_compile_variables,
        ) + ctx.fragments.cpp.copts + ctx.fragments.cpp.cxxopts,
        _COMPILER_OPTIONS_DENYLIST,
    )
    env.update(cc_common.get_environment_variables(
        feature_configuration = feature_configuration,
        action_name = CPP_COMPILE_ACTION_NAME,
        variables = cxx_compile_variables,
    ))

    objc_compile_variables = cc_common.create_compile_variables(
        feature_configuration = feature_configuration,
        cc_toolchain = cc_toolchain,
    )
    objc_compile_options = _filter_options(
        cc_common.get_memory_inefficient_command_line(
            feature_configuration = feature_configuration,
            action_name = OBJC_COMPILE_ACTION_NAME,
            variables = objc_compile_variables,
        ),
        _COMPILER_OPTIONS_DENYLIST,
    )
    env.update(cc_common.get_environment_variables(
        feature_configuration = feature_configuration,
        action_name = OBJC_COMPILE_ACTION_NAME,
        variables = objc_compile_variables,
    ))

    objcxx_compile_variables = cc_common.create_compile_variables(
        feature_configuration = feature_configuration,
        cc_toolchain = cc_toolchain,
    )
    objcxx_compile_options = _filter_options(
        cc_common.get_memory_inefficient_command_line(
            feature_configuration = feature_configuration,
            action_name = OBJCPP_COMPILE_ACTION_NAME,
            variables = objcxx_compile_variables,
        ),
        _COMPILER_OPTIONS_DENYLIST,
    )
    env.update(cc_common.get_environment_variables(
        feature_configuration = feature_configuration,
        action_name = OBJCPP_COMPILE_ACTION_NAME,
        variables = objcxx_compile_variables,
    ))

    ld_executable_variables = cc_common.create_link_variables(
        feature_configuration = feature_configuration,
        cc_toolchain = cc_toolchain,
        is_linking_dynamic_library = False,
    )
    ld_executable_path = cc_common.get_tool_for_action(
        feature_configuration = feature_configuration,
        action_name = CPP_LINK_EXECUTABLE_ACTION_NAME,
    )
    ld_executable_options = _filter_options(
        cc_common.get_memory_inefficient_command_line(
            feature_configuration = feature_configuration,
            action_name = CPP_LINK_EXECUTABLE_ACTION_NAME,
            variables = ld_executable_variables,
        ) + ctx.fragments.cpp.linkopts,
        _LINKER_OPTIONS_DENYLIST,
    )
    env.update(cc_common.get_environment_variables(
        feature_configuration = feature_configuration,
        action_name = CPP_LINK_EXECUTABLE_ACTION_NAME,
        variables = ld_executable_variables,
    ))

    # We don't collect options for static libraries. Go always links with
    # "ar" in "c-archive" mode. We can set the ar executable path with
    # -extar, but the options are hard-coded to something like -q -c -s.
    ld_static_lib_variables = cc_common.create_link_variables(
        feature_configuration = feature_configuration,
        cc_toolchain = cc_toolchain,
        is_linking_dynamic_library = False,
    )
    ld_static_lib_path = cc_common.get_tool_for_action(
        feature_configuration = feature_configuration,
        action_name = CPP_LINK_STATIC_LIBRARY_ACTION_NAME,
    )
    env.update(cc_common.get_environment_variables(
        feature_configuration = feature_configuration,
        action_name = CPP_LINK_STATIC_LIBRARY_ACTION_NAME,
        variables = ld_static_lib_variables,
    ))

    ld_dynamic_lib_variables = cc_common.create_link_variables(
        feature_configuration = feature_configuration,
        cc_toolchain = cc_toolchain,
        is_linking_dynamic_library = True,
    )
    ld_dynamic_lib_path = cc_common.get_tool_for_action(
        feature_configuration = feature_configuration,
        action_name = CPP_LINK_DYNAMIC_LIBRARY_ACTION_NAME,
    )
    ld_dynamic_lib_options = _filter_options(
        cc_common.get_memory_inefficient_command_line(
            feature_configuration = feature_configuration,
            action_name = CPP_LINK_DYNAMIC_LIBRARY_ACTION_NAME,
            variables = ld_dynamic_lib_variables,
        ) + ctx.fragments.cpp.linkopts,
        _LINKER_OPTIONS_DENYLIST,
    )

    env.update(cc_common.get_environment_variables(
        feature_configuration = feature_configuration,
        action_name = CPP_LINK_DYNAMIC_LIBRARY_ACTION_NAME,
        variables = ld_dynamic_lib_variables,
    ))

    apple_ensure_options(
        ctx,
        env,
        (c_compile_options, cxx_compile_options, objc_compile_options, objcxx_compile_options),
        (ld_executable_options, ld_dynamic_lib_options),
        cc_toolchain.target_gnu_system_name,
    )

    # Add C toolchain directories to PATH.
    # On ARM, go tool link uses some features of gcc to complete its work,
    # so PATH is needed on ARM.
    path_set = {}
    if "PATH" in env:
        for p in env["PATH"].split(ctx.configuration.host_path_separator):
            path_set[p] = None
    tool_paths = [
        c_compiler_path,
        ld_executable_path,
        ld_static_lib_path,
        ld_dynamic_lib_path,
    ]
    for tool_path in tool_paths:
        tool_dir = tool_path[:tool_path.rfind("/")]
        path_set[tool_dir] = None
    paths = path_set.keys()
    if ctx.configuration.host_path_separator == ":":
        # HACK: ":" is a proxy for a UNIX-like host.
        # The tools returned above may be bash scripts that reference commands
        # in directories we might not otherwise include. For example,
        # on macOS, wrapped_ar calls dirname.
        if "/bin" not in path_set:
            paths.append("/bin")
        if "/usr/bin" not in path_set:
            paths.append("/usr/bin")
    env["PATH"] = ctx.configuration.host_path_separator.join(paths)

    return CgoContextInfo(
        cc_toolchain_files = cc_toolchain.all_files,
        env = env,
        cgo_tools = struct(
            cc_toolchain = cc_toolchain,
            feature_configuration = feature_configuration,
            c_compiler_path = c_compiler_path,
            c_compile_options = c_compile_options,
            cxx_compile_options = cxx_compile_options,
            objc_compile_options = objc_compile_options,
            objcxx_compile_options = objcxx_compile_options,
            ld_executable_path = ld_executable_path,
            ld_executable_options = ld_executable_options,
            ld_static_lib_path = ld_static_lib_path,
            ld_dynamic_lib_path = ld_dynamic_lib_path,
            ld_dynamic_lib_options = ld_dynamic_lib_options,
            ar_path = cc_toolchain.ar_executable,
        ),
    )

cgo_context_data = rule(
    implementation = cgo_context_data_impl,
    attrs = CGO_ATTRS,
    toolchains = CGO_TOOLCHAINS,
    fragments = CGO_FRAGMENTS,
    doc = """Collects information about the C/C++ toolchain. The C/C++ toolchain
    is needed to build cgo code, but is generally optional. Rules can't have
    optional toolchains, so instead, we have an optional dependency on this
    rule.""",
)

def _cgo_context_data_proxy_impl(ctx):
    if ctx.attr.actual and CgoContextInfo in ctx.attr.actual:
        return [ctx.attr.actual[CgoContextInfo]]
    return []

cgo_context_data_proxy = rule(
    implementation = _cgo_context_data_proxy_impl,
    attrs = {
        "actual": attr.label(),
    },
    doc = """Conditionally depends on cgo_context_data and forwards it provider.

    Useful in situations where select cannot be used, like attribute defaults.
    """,
)

def _go_config_impl(ctx):
    pgo_profiles = ctx.attr.pgoprofile.files.to_list()
    if len(pgo_profiles) > 2:
        fail("providing more than one pprof file to pgoprofile is not supported")
    if len(pgo_profiles) == 1:
        pgoprofile = pgo_profiles[0]
    else:
        pgoprofile = None

    tags = list(ctx.attr.gotags[BuildSettingInfo].value)
    if "gotags" in ctx.var:
        tags += ctx.var["gotags"].split(",")

    race = ctx.attr.race[BuildSettingInfo].value
    if race:
        tags.append("race")

    msan = ctx.attr.msan[BuildSettingInfo].value
    if msan:
        tags.append("msan")

    toolchain = ctx.toolchains[GO_TOOLCHAIN]

    go_config_info = GoConfigInfo(
        goos = toolchain.default_goos,
        goarch = toolchain.default_goarch,
        static = ctx.attr.static[BuildSettingInfo].value,
        race = race,
        msan = msan,
        pure = ctx.attr.pure[BuildSettingInfo].value,
        strip = ctx.attr.strip,
        debug = ctx.attr.debug[BuildSettingInfo].value,
        linkmode = ctx.attr.linkmode[BuildSettingInfo].value,
        gc_linkopts = ctx.attr.gc_linkopts[BuildSettingInfo].value,
        tags = tags,
        stamp = ctx.attr.stamp,
        cover_format = ctx.attr.cover_format[BuildSettingInfo].value,
        gc_goopts = ctx.attr.gc_goopts[BuildSettingInfo].value,
        amd64 = ctx.attr.amd64,
        arm = ctx.attr.arm,
        pgoprofile = pgoprofile,
        export_stdlib = ctx.attr.export_stdlib[BuildSettingInfo].value,
    )
    validate_mode(go_config_info)

    return [go_config_info]

go_config = rule(
    implementation = _go_config_impl,
    attrs = {
        "static": attr.label(
            mandatory = True,
            providers = [BuildSettingInfo],
        ),
        "race": attr.label(
            mandatory = True,
            providers = [BuildSettingInfo],
        ),
        "msan": attr.label(
            mandatory = True,
            providers = [BuildSettingInfo],
        ),
        "pure": attr.label(
            mandatory = True,
            providers = [BuildSettingInfo],
        ),
        "strip": attr.bool(mandatory = True),
        "debug": attr.label(
            mandatory = True,
            providers = [BuildSettingInfo],
        ),
        "linkmode": attr.label(
            mandatory = True,
            providers = [BuildSettingInfo],
        ),
        "gc_linkopts": attr.label(
            mandatory = True,
            providers = [BuildSettingInfo],
        ),
        "gotags": attr.label(
            mandatory = True,
            providers = [BuildSettingInfo],
        ),
        "stamp": attr.bool(mandatory = True),
        "cover_format": attr.label(
            mandatory = True,
            providers = [BuildSettingInfo],
        ),
        "gc_goopts": attr.label(
            mandatory = True,
            providers = [BuildSettingInfo],
        ),
        "amd64": attr.string(),
        "arm": attr.string(),
        "pgoprofile": attr.label(
            mandatory = True,
            allow_files = True,
        ),
        "export_stdlib": attr.label(
            mandatory = False,
            providers = [BuildSettingInfo],
        ),
    },
    provides = [GoConfigInfo],
    doc = """Collects information about build settings in the current
    configuration. Rules may depend on this instead of depending on all
    the build settings directly.""",
    toolchains = [GO_TOOLCHAIN],
)

def _expand_opts(go, attribute_name, opts):
    return [go._ctx.expand_make_variables(attribute_name, opt, {}) for opt in opts]
