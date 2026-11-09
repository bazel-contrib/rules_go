# Copyright 2014 The Bazel Authors. All rights reserved.
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
    "@bazel_skylib//lib:collections.bzl",
    "collections",
)
load(
    "//go/private:common.bzl",
    "GO_TOOLCHAIN_LABEL",
    "count_group_matches",
    "has_shared_lib_extension",
)
load(
    "//go/private:mode.bzl",
    "LINKMODE_NORMAL",
    "LINKMODE_PIE",
    "LINKMODE_PLUGIN",
    "extld_from_cc_toolchain",
    "extldflags_from_cc_toolchain",
)
load(
    "//go/private:providers.bzl",
    "archive_uses_cgo",
    "package_uses_cgo",
)
load(
    "//go/private:rpath.bzl",
    "rpath",
)
load("//go/private:sdk.bzl", "parse_version")

# Go linker flags that need the external linker or that change the link mode.
# Each flag also matches in its "-flag=value" form.
_EXTERNAL_LINKOPT_EXACT = (
    "-I",
    "-r",
    "-linkmode",
    "-extld",
    "-extldflags",
    "-extar",
    "-linkshared",
    "-buildmode",
    "-race",
    "-msan",
    "-asan",
)

_EXTERNAL_LINKOPT_PREFIXES = (
    "-I=",
    "-r=",
    "-linkmode=",
    "-extld=",
    "-extldflags=",
    "-extar=",
    "-linkshared=",
    "-buildmode=",
    "-race=",
    "-msan=",
    "-asan=",
)

# The GOOS values that satisfy the "unix" build constraint. This is UnixOS in
# Go's internal/syslist package.
_UNIX_GOOS = (
    "aix",
    "android",
    "darwin",
    "dragonfly",
    "freebsd",
    "hurd",
    "illumos",
    "ios",
    "linux",
    "netbsd",
    "openbsd",
    "solaris",
)

def _is_external_linkopt(opt):
    if opt.startswith("--"):
        opt = opt[1:]
    return opt in _EXTERNAL_LINKOPT_EXACT or opt.startswith(_EXTERNAL_LINKOPT_PREFIXES)

def stdlib_uses_cgo(goos, goarch, tags, experiments):
    """Returns whether the standard library can link cgo code into a binary.

    This mirrors the build constraints of the standard library files that
    import "C". The stdlib_cgo_predicate_test in tests/core/go_binary compares
    it with "go list" for each platform, so it fails when Go changes them.

    The plugin package is not modeled, because analysis cannot see which
    packages a binary imports. By default, the Go linker still links a plugin
    host externally. With //go/config:internal_link, a binary that imports
    plugin must set pure = "off". The race and msan modes cover runtime/race
    and runtime/msan.

    Go reads the boringcrypto build constraint as goexperiment.boringcrypto
    (matchTag in go/build/build.go). So the boringcrypto tag alone does not
    turn on BoringCrypto, but the goexperiment.boringcrypto tag does.

    Args:
      goos: The target GOOS.
      goarch: The target GOARCH.
      tags: The build tags of the standard library.
      experiments: The GOEXPERIMENT value as a comma-separated string. It can
        be empty or None.

    Returns:
      True if the standard library can link cgo code.
    """
    if not experiments:
        experiments = []
    elif type(experiments) == "string":
        experiments = experiments.split(",")
    unix = goos in _UNIX_GOOS

    # net/cgo_unix_cgo.go: "cgo && !netgo && unix && !darwin". GOOS=ios satisfies the darwin tag.
    if unix and goos not in ("darwin", "ios") and "netgo" not in tags:
        return True

    # os/user/cgo_lookup_cgo.go: "cgo && !osusergo && unix && !android && !darwin".
    if unix and goos not in ("android", "darwin", "ios") and "osusergo" not in tags:
        return True

    # crypto/internal/boring/boring.go: "boringcrypto && linux && (amd64 || arm64) && !android".
    if ("goexperiment.boringcrypto" in tags or "boringcrypto" in experiments) and goos == "linux" and goarch in ("amd64", "arm64"):
        return True

    # runtime/asan/asan.go links the C address sanitizer runtime.
    if "asan" in tags:
        return True

    return False

def _must_link_external(goos, goarch):
    # This mirrors internal/platform.MustLinkExternal(goos, goarch, false) in Go 1.26.
    if goos == "android":
        return goarch != "arm64"
    return goos == "ios" and goarch == "arm64"

def _static_needs_external_linker(goos, goarch):
    # The internal linker cannot link libc statically. On these platforms, the
    # syscall package imports libc dynamically with //go:cgo_import_dynamic in
    # syscall/zsyscall_*.go. On openbsd, this is true for each GOARCH except
    # mips64 (in Go 1.26, syscall/syscall_openbsd_libc.go has
    # "openbsd && !mips64"). darwin and windows use system libraries too, but
    # static has no effect on Mach-O and PE files, so an internal link is
    # correct there (#2231).
    if goos in ("aix", "illumos", "solaris"):
        return True
    return goos == "openbsd" and goarch != "mips64"

def _internal_link_pie_supported(goos, goarch):
    # These match Go's internal/platform.InternalLinkPIESupported for non-ELF
    # formats. ELF PIE stays external for two reasons. An internally linked ELF
    # PIE takes its interpreter path from the build host
    # (cmd/link/internal/ld/elf.go). And it cannot be static, because cmd/link
    # sets -d only for buildmode exe (cmd/link/internal/ld/lib.go).
    if goos == "darwin":
        return goarch in ("amd64", "arm64")
    if goos == "windows":
        return goarch in ("386", "amd64", "arm64")
    return False

def _may_link_internally(go, archive, test_archives, link_flags):
    """Returns whether a binary that is not pure can link without the C/C++ linker.

    Analysis cannot see which standard library packages a binary imports. A
    binary that imports plugin passes these checks, but it still needs the
    external linker.

    Args:
      go: The go object returned by go_context.
      archive: The GoArchive of the main package.
      test_archives: GoArchiveData for test packages linked into the binary.
      link_flags: The Go linker flags from the rule, the mode, and the
        toolchain, before -extldflags values are split out.

    Returns:
      True if no check below needs the external linker.
    """

    # pure = "off" asks for the C/C++ toolchain, and go_context fails without one (context.bzl).
    if go._explicit_pure_off:
        return False

    # The race and msan runtimes need cgo and the external linker (#2628, #3370).
    if go.mode.race or go.mode.msan:
        return False

    # cgo code and .syso objects can need the external linker (cmd/link/internal/ld/config.go).
    if archive_uses_cgo(archive) or any([package_uses_cgo(d) for d in test_archives]):
        return False

    # The standard library links cgo code on some platforms. stdlib_uses_cgo cites the Go files.
    if stdlib_uses_cgo(go.mode.goos, go.mode.goarch, go.mode.tags, go.sdk.experiments):
        return False

    # Go requires the external linker on these platforms (internal/platform.MustLinkExternal).
    if _must_link_external(go.mode.goos, go.mode.goarch):
        return False

    if go.mode.linkmode == LINKMODE_NORMAL:
        # A static binary keeps the external linker where syscall uses libc (_static_needs_external_linker).
        if go.static_link and _static_needs_external_linker(go.mode.goos, go.mode.goarch):
            return False
    elif go.mode.linkmode == LINKMODE_PIE:
        # PIE links internally only for Mach-O and PE files (_internal_link_pie_supported).
        if not _internal_link_pie_supported(go.mode.goos, go.mode.goarch):
            return False
    else:
        # plugin, c-shared, c-archive, and shared need the external linker (cmd/link/internal/ld/config.go).
        return False

    # A Go linker flag selects the external linker or a link mode (cmd/link/internal/ld/main.go).
    if any([_is_external_linkopt(f) for f in link_flags]):
        return False

    # --linkopt values are flags for the C/C++ linker (ld_executable_options in context.bzl).
    if getattr(go.cgo_tools, "user_linkopts", None):
        return False

    # Before Go 1.24, internal links have no GNU build ID or LC_UUID. macOS needs LC_UUID (golang/go#68678).
    sdk_version = parse_version(go.sdk.version)
    if not sdk_version or sdk_version[:2] < (1, 24):
        return False

    return True

def _format_archive(d):
    return "{}={}={}".format(d.label, d.importmap, d.file.path)

def _format_package_metadata(d):
    if not d._package_metadata:
        return None
    return "{}={}".format(d.importmap, d._package_metadata.path)

def _format_imports(d, main_archive_file):
    if d.file == main_archive_file:
        return None
    return "{}={}".format(d.importmap, d._imports.path)

def emit_link(
        go,
        archive = None,
        test_archives = [],
        executable = None,
        gc_linkopts = [],
        version_file = None,
        info_file = None,
        exec_group = None,
        buildinfo_main_package_path = None,
        buildinfo_module_metadata = None):
    """See go/toolchains.rst#link for full documentation."""

    if archive == None:
        fail("archive is a required parameter")
    if executable == None:
        fail("executable is a required parameter")
    if buildinfo_main_package_path == None:
        buildinfo_main_package_path = archive.data.importpath
    if buildinfo_module_metadata == None:
        buildinfo_module_metadata = getattr(archive.data, "_package_metadata", None)

    # Exclude -lstdc++ from link options. We don't want to link against it
    # unless we actually have some C++ code. _cgo_codegen will include it
    # in archives via CGO_LDFLAGS if it's needed.
    extldflags = [f for f in extldflags_from_cc_toolchain(go) if f not in ("-lstdc++", "-lc++", "-static")]

    if go.coverage_enabled:
        extldflags.append("--coverage")
    gc_linkopts = gc_linkopts + go.mode.gc_linkopts

    # GoLink selects the linker in one of three ways:
    # * Internal: rules_go passes -linkmode internal and leaves out the C/C++
    #   toolchain. This happens in pure mode (#2743), and for a binary that can
    #   link internally when //go/config:internal_link is set.
    # * Automatic: rules_go passes no -linkmode flag for a binary that can link
    #   internally. The Go linker then decides from the objects it links, as
    #   with go build. It links a plugin host externally, for example.
    # * External: in the other cases, rules_go forces the external linker
    #   under the conditions listed below.
    may_link_internally = not go.mode.pure and _may_link_internally(go, archive, test_archives, gc_linkopts + list(go.toolchain.flags.link))
    use_internal_link = go.mode.pure or (may_link_internally and getattr(go.mode, "internal_link", False))
    gc_linkopts, extldflags = _extract_extldflags(gc_linkopts, extldflags)
    builder_args = go.builder_args(go, "link")
    tool_args = go.tool_args(go)

    # use ar tool from cc toolchain if cc toolchain provides it
    if not use_internal_link and go.cgo_tools and go.cgo_tools.ar_path and go.cgo_tools.ar_path.endswith("ar"):
        tool_args.add("-extar", go.cgo_tools.ar_path)

    # Add in any mode specific behaviours
    if go.mode.race:
        builder_args.add("-race")
        tool_args.add("-race")
    if go.mode.msan:
        builder_args.add("-msan")
        tool_args.add("-msan")

    if use_internal_link:
        tool_args.add("-linkmode", "internal")
    else:
        extld = extld_from_cc_toolchain(go)
        tool_args.add_all(extld)
        if extld and not may_link_internally and (go.static_link or
                                                  go.mode.race or
                                                  go.mode.linkmode != LINKMODE_NORMAL or
                                                  go.mode.goos == "windows" and go.mode.msan):
            # Force external linking for the following conditions, unless the
            # binary can link internally (_may_link_internally):
            # * Mode is static but not pure: -static must be passed to the C
            #   linker if the binary contains cgo code. See #2168, #2216.
            # * Non-normal build mode: may not be strictly necessary, especially
            #   for modes like "pie".
            # * Race or msan build for Windows: Go linker has pairwise
            #   incompatibilities with mingw, and we get link errors in race mode.
            #   Using the C linker avoids that. Race and msan always require a
            #   a C toolchain. See #2614.
            # * Linux race builds: we get linker errors during build with Go's
            #   internal linker. For example, when using zig cc v0.10
            #   (clang-15.0.3):
            #
            #       runtime/cgo(.text): relocation target memset not defined
            tool_args.add("-linkmode", "external")

    if go.static_link:
        extldflags.append("-static")
    if go.mode.linkmode != LINKMODE_NORMAL:
        builder_args.add("-buildmode", go.mode.linkmode)
    if go.mode.linkmode == LINKMODE_PLUGIN:
        tool_args.add("-pluginpath", archive.data.importpath)

    arcs = depset(
        test_archives + ([go.coverdata.data] if go.coverage_enabled and go.coverdata else []),
        transitive = [d.transitive for d in archive.direct],
    )

    all_archive_data = depset(test_archives, transitive = [archive.transitive])
    buildinfo_link_inputs = depset(
        direct = [
            metadata
            for archive_data in test_archives
            for metadata in [getattr(archive_data, "_package_metadata", None)]
            if metadata
        ] + [archive_data._imports for archive_data in test_archives],
        transitive = [getattr(archive, "_buildinfo_link_inputs", depset())],
    )

    builder_args.add_all(arcs, before_each = "-arc", map_each = _format_archive)
    builder_args.add_all(all_archive_data, before_each = "-package_metadata", map_each = _format_package_metadata)
    builder_args.add_all(
        all_archive_data,
        before_each = "-imports",
        map_each = lambda d: _format_imports(d, archive.data.file),
        allow_closure = True,
    )
    builder_args.add("-package_list", go.sdk.package_list)
    if go.coverage_enabled:
        builder_args.add("-cover")

    # Build a list of rpaths for dynamic libraries we need to find.
    # rpaths are relative paths from the binary to directories where libraries
    # are stored. Binaries that require these will only work when installed in
    # the bazel execroot. Most binaries are only dynamically linked against
    # system libraries though.
    cgo_rpaths = sorted(collections.uniq([
        f
        for d in archive.cgo_deps.to_list()
        if has_shared_lib_extension(d.basename)
        for f in rpath.flags(go, d, executable = executable)
    ]))
    extldflags.extend(cgo_rpaths)

    # Process x_defs, and record whether stamping is used.
    stamp_x_defs_volatile = False
    stamp_x_defs_stable = False
    for k, v in archive.x_defs.items():
        builder_args.add("-X", "%s=%s" % (k, v))
        if go.mode.stamp:
            stable_vars_count = (count_group_matches(v, "{STABLE_", "}") +
                                 v.count("{BUILD_EMBED_LABEL}") +
                                 v.count("{BUILD_USER}") +
                                 v.count("{BUILD_HOST}"))
            if stable_vars_count > 0:
                stamp_x_defs_stable = True
            if count_group_matches(v, "{", "}") != stable_vars_count:
                stamp_x_defs_volatile = True

    # Stamping support
    stamp_inputs = []
    if stamp_x_defs_stable:
        stamp_inputs.append(info_file)
    if stamp_x_defs_volatile:
        stamp_inputs.append(version_file)
    if stamp_inputs:
        builder_args.add_all(stamp_inputs, before_each = "-stamp")

    builder_args.add("-o", executable)
    builder_args.add("-main", archive.data.file)
    builder_args.add("-main_package_path", buildinfo_main_package_path)
    builder_args.add("-main_imports", archive.data._imports)
    if buildinfo_module_metadata:
        builder_args.add("-main_module_metadata", buildinfo_module_metadata)
    builder_args.add("-p", archive.data.importmap)
    tool_args.add_all(gc_linkopts)
    tool_args.add_all(go.toolchain.flags.link)

    sdk_version = parse_version(go.sdk.version)
    if go.mode.goos == "darwin" and sdk_version and sdk_version[:2] >= (1, 27) and go.macos_minimum_os:
        tool_args.add("-macos", go.macos_minimum_os)

    if go.mode.strip:
        tool_args.add("-s", "-w")

    # The Go linker reads -extldflags only when it links externally. Pure mode
    # keeps them, because a -linkmode flag in gc_linkopts can still select the
    # external linker.
    if go.mode.pure or not use_internal_link:
        tool_args.add_joined("-extldflags", extldflags, join_with = " ")

    inputs_direct = stamp_inputs + [go.sdk.package_list]
    if buildinfo_module_metadata:
        inputs_direct.append(buildinfo_module_metadata)
    if go.coverage_enabled and go.coverdata:
        inputs_direct.append(go.coverdata.data.file)
    inputs_transitive = [
        archive.libs,
        archive.cgo_deps,
        archive.cgo_link_inputs,
        go.sdk.tools,
        go.stdlib.libs,
        buildinfo_link_inputs,
    ]

    # The internal linker runs no C/C++ tools, so it needs no C/C++ toolchain files.
    if not use_internal_link:
        inputs_transitive.extend([
            go.stdlib.cgo_link_inputs,
            go.cc_toolchain_files,
        ])
    inputs = depset(direct = inputs_direct, transitive = inputs_transitive)

    go.actions.run(
        inputs = inputs,
        outputs = [executable],
        mnemonic = "GoLink",
        executable = go.toolchain._builder,
        arguments = [builder_args, "--", tool_args],
        env = go._env_without_cc if use_internal_link else go.env,
        toolchain = GO_TOOLCHAIN_LABEL,
        exec_group = exec_group,
    )

def _extract_extldflags(gc_linkopts, extldflags):
    """Extracts -extldflags from gc_linkopts and combines them into a single list.

    Args:
      gc_linkopts: a list of flags passed in through the gc_linkopts attributes.
        ctx.expand_make_variables should have already been applied. -extldflags
        may appear multiple times in this list.
      extldflags: a list of flags to be passed to the external linker.

    Return:
      A tuple containing the filtered gc_linkopts with external flags removed,
      and a combined list of external flags. Each string in the returned
      extldflags list may contain multiple flags, separated by whitespace.
    """
    filtered_gc_linkopts = []
    is_extldflags = False
    skip_next = False

    for i, opt in enumerate(gc_linkopts):
        if skip_next:
            skip_next = False
            continue

        if is_extldflags:
            if opt == "-Wl" and i + 1 < len(gc_linkopts):
                # Merge '-Wl,' and next value
                extldflags.append("-Wl," + gc_linkopts[i + 1])
                skip_next = True
            else:
                extldflags.append(opt)
            is_extldflags = False
        elif opt == "-extldflags":
            is_extldflags = True
        else:
            filtered_gc_linkopts.append(opt)
    return filtered_gc_linkopts, extldflags
