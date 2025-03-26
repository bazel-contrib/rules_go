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
    "LINKMODE_PLUGIN",
    "extld_from_cc_toolchain",
    "extldflags_from_cc_toolchain",
)
load(
    "//go/private:rpath.bzl",
    "rpath",
)

def _format_archive(d):
    return "{}={}={}".format(d.label, d.importmap, d.file.path)

def emit_link(
        go,
        archive = None,
        test_archives = [],
        executable = None,
        gc_linkopts = [],
        version_file = None,
        info_file = None):
    """See go/toolchains.rst#link for full documentation."""

    if archive == None:
        fail("archive is a required parameter")
    if executable == None:
        fail("executable is a required parameter")

    # Exclude -lstdc++ from link options. We don't want to link against it
    # unless we actually have some C++ code. _cgo_codegen will include it
    # in archives via CGO_LDFLAGS if it's needed.
    extldflags = [f for f in extldflags_from_cc_toolchain(go) if f not in ("-lstdc++", "-lc++", "-static")]

    if go.coverage_enabled:
        extldflags.append("--coverage")
    gc_linkopts = gc_linkopts + go.mode.gc_linkopts
    gc_linkopts, extldflags = _extract_extldflags(gc_linkopts, extldflags)
    builder_args = go.builder_args(go, "link")
    tool_args = go.tool_args(go)

    # use ar tool from cc toolchain if cc toolchain provides it
    if go.cgo_tools and go.cgo_tools.ar_path and go.cgo_tools.ar_path.endswith("ar"):
        tool_args.add_all(["-extar", go.cgo_tools.ar_path])

    # Add in any mode specific behaviours
    if go.mode.race:
        tool_args.add("-race")
    if go.mode.msan:
        tool_args.add("-msan")

    if go.mode.pure:
        tool_args.add("-linkmode", "internal")
    else:
        extld = extld_from_cc_toolchain(go)
        tool_args.add_all(extld)
        if extld and (go.mode.static or
                      go.mode.race or
                      go.mode.linkmode != LINKMODE_NORMAL or
                      go.mode.goos == "windows" and go.mode.msan):
            # Force external linking for the following conditions:
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

    if go.mode.static:
        extldflags.append("-static")
    if go.mode.linkmode != LINKMODE_NORMAL:
        builder_args.add("-buildmode", go.mode.linkmode)
    if go.mode.linkmode == LINKMODE_PLUGIN:
        tool_args.add("-pluginpath", archive.data.importpath)

    # TODO(zbarsky): Bazel versions older than 7.2.0 do not properly deduplicate this dep
    # Can replace with the following once we support Bazel 7.2.0+ only:
    #    if go.coverage_enabled and go.coverdata:
    #        test_archives = list(test_archives) + [go.coverdata.data]
    #    arcs = depset(test_archives, transitive = [d.transitive for d in archive.direct])

    if go.coverage_enabled and go.coverdata:
        potentially_duplicated_arcs = depset(test_archives + [go.coverdata.data], transitive = [d.transitive for d in archive.direct]).to_list()
        importmaps = {}
        arcs = []
        for arc in potentially_duplicated_arcs:
            if arc.importmap in importmaps:
                continue
            importmaps[arc.importmap] = True
            arcs.append(arc)
    else:
        arcs = depset(test_archives, transitive = [d.transitive for d in archive.direct])

    builder_args.add_all(arcs, before_each = "-arc", map_each = _format_archive)
    builder_args.add("-package_list", go.sdk.package_list)

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

    # BUG: Have we forgotten about the .a files?
    for f in archive.cgo_deps.to_list():
        extldflags.extend([
            "-L", f.dirname,
            # This shenaningan converts "libzmq.a" into "-lzmq" as one would
            # want in a CC command line.
            "-l{}".format(f.basename[3:-2])]
        )
    # We will need to link with stdc++ too if we want C++ stdlib to work.
    # This is probably not correct for C programs. What to do?
    extldflags.extend(["-lstdc++"])

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
    builder_args.add("-p", archive.data.importmap)
    tool_args.add_all(gc_linkopts)
    tool_args.add_all(go.toolchain.flags.link)

    # Do not remove, somehow this is needed when building for darwin/arm only.
    tool_args.add("-buildid=redacted")
    if go.mode.strip:
        tool_args.add("-s", "-w")
    tool_args.add_joined("-extldflags", extldflags, join_with = " ")

    inputs_direct = stamp_inputs + [go.sdk.package_list]
    if go.coverage_enabled and go.coverdata:
        inputs_direct.append(go.coverdata.data.file)
    inputs_transitive = [
        archive.libs,
        archive.cgo_deps,
        go.cc_toolchain_files,
        go.sdk.tools,
        go.stdlib.libs,
    ]
    inputs = depset(direct = inputs_direct, transitive = inputs_transitive)


    go.actions.run(
        inputs = inputs,
        outputs = [executable],
        mnemonic = "GoLink",
        executable = go.toolchain._builder,
        arguments = [builder_args, "--", tool_args],
        env = go.env,
        toolchain = GO_TOOLCHAIN_LABEL,
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
    for opt in gc_linkopts:
        if is_extldflags:
            is_extldflags = False
            extldflags.append(opt)
        elif opt == "-extldflags":
            is_extldflags = True
        else:
            filtered_gc_linkopts.append(opt)
    return filtered_gc_linkopts, extldflags
