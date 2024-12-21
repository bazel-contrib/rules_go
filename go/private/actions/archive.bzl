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
    "//go/private:context.bzl",
    "get_nogo",
)
load(
    "//go/private:mode.bzl",
    "LINKMODE_C_ARCHIVE",
    "LINKMODE_C_SHARED",
    "mode_string",
)
load(
    "//go/private:providers.bzl",
    "GoArchive",
    "GoArchiveData",
    "effective_importpath_pkgpath",
)
load(
    "//go/private/actions:compilepkg.bzl",
    "emit_compilepkg",
)
load(
    "//go/private/rules:cgo.bzl",
    "cgo_configure",
)

def emit_archive(go, source = None, _recompile_suffix = "", recompile_internal_deps = None, is_external_pkg = False):
    """See go/toolchains.rst#archive for full documentation."""

    if source == None:
        fail("source is a required parameter")

    testfilter = getattr(source, "testfilter", None)
    pre_ext = ""
    if go.mode.linkmode == LINKMODE_C_ARCHIVE:
        pre_ext = "_"  # avoid collision with go_binary output file with .a extension
    elif testfilter == "exclude":
        pre_ext = ".internal"
    elif testfilter == "only":
        pre_ext = ".external"
    if _recompile_suffix:
        pre_ext += _recompile_suffix
    out_lib = go.declare_file(go, name = source.name, ext = pre_ext + ".a")

    # store export information for compiling dependent packages separately
    out_export = go.declare_file(go, name = source.name, ext = pre_ext + ".x")
    out_cgo_export_h = None  # set if cgo used in c-shared or c-archive mode

    nogo = get_nogo(go)
    if nogo:
        out_facts = go.declare_file(go, name = source.name, ext = pre_ext + ".facts")
        out_nogo_log = go.declare_file(go, name = source.name, ext = pre_ext + ".nogo.log")
        out_nogo_validation = go.declare_file(go, name = source.name, ext = pre_ext + ".nogo")
        out_nogo_fix = go.declare_file(go, name = source.name, ext = pre_ext + ".nogo.patch")
    else:
        out_facts = None
        out_nogo_log = None
        out_nogo_validation = None
        out_nogo_fix = None

    direct = source.deps

    files = []
    for a in direct:
        files.append(a.runfiles)
        if a.source.mode != go.mode:
            fail("Archive mode does not match {} is {} expected {}".format(a.data.label, mode_string(a.source.mode), mode_string(go.mode)))
    runfiles = source.runfiles.merge_all(files)

    importmap = "main" if source.is_main else source.importmap
    importpath, _ = effective_importpath_pkgpath(source)

    if source.cgo and not go.mode.pure:
        # TODO(jayconrod): do we need to do full Bourne tokenization here?
        cppopts = [f for fs in source.cppopts for f in fs.split(" ")]
        copts = [f for fs in source.copts for f in fs.split(" ")]
        cxxopts = [f for fs in source.cxxopts for f in fs.split(" ")]
        clinkopts = [f for fs in source.clinkopts for f in fs.split(" ")]
        cgo = cgo_configure(
            go,
            srcs = source.srcs,
            cdeps = source.cdeps,
            cppopts = cppopts,
            copts = copts,
            cxxopts = cxxopts,
            clinkopts = clinkopts,
        )
        if go.mode.linkmode in (LINKMODE_C_SHARED, LINKMODE_C_ARCHIVE):
            out_cgo_export_h = go.declare_file(go, path = "_cgo_install.h")
        cgo_deps = cgo.deps
        runfiles = runfiles.merge(cgo.runfiles)
        emit_compilepkg(
            go,
            sources = source.srcs,
            cover = source.cover,
            embedsrcs = source.embedsrcs,
            importpath = importpath,
            importmap = importmap,
            archives = direct,
            out_lib = out_lib,
            out_export = out_export,
            out_facts = out_facts,
            out_nogo_log = out_nogo_log,
            out_nogo_validation = out_nogo_validation,
            out_nogo_fix = out_nogo_fix,
            nogo = nogo,
            out_cgo_export_h = out_cgo_export_h,
            gc_goopts = source.gc_goopts,
            cgo = True,
            cgo_inputs = cgo.inputs,
            cppopts = cgo.cppopts,
            copts = cgo.copts,
            cxxopts = cgo.cxxopts,
            objcopts = cgo.objcopts,
            objcxxopts = cgo.objcxxopts,
            clinkopts = cgo.clinkopts,
            testfilter = testfilter,
            is_external_pkg = is_external_pkg,
        )
    else:
        cgo_deps = depset()
        emit_compilepkg(
            go,
            sources = source.srcs,
            cover = source.cover,
            embedsrcs = source.embedsrcs,
            importpath = importpath,
            importmap = importmap,
            archives = direct,
            out_lib = out_lib,
            out_export = out_export,
            out_facts = out_facts,
            out_nogo_log = out_nogo_log,
            out_nogo_validation = out_nogo_validation,
            out_nogo_fix = out_nogo_fix,
            nogo = nogo,
            gc_goopts = source.gc_goopts,
            cgo = False,
            testfilter = testfilter,
            recompile_internal_deps = recompile_internal_deps,
            is_external_pkg = is_external_pkg,
        )

    data = GoArchiveData(
        # TODO(#2578): reconsider the provider API. There's a lot of redundant
        # information here. Some fields are tuples instead of lists or dicts
        # since GoArchiveData is stored in a depset, and no value in a depset
        # may be mutable. For now, new copied fields are private (named with
        # a leading underscore) since they may change in the future.

        # GoInfo fields
        name = source.name,
        label = source.label,
        importpath = source.importpath,
        importmap = source.importmap,
        importpath_aliases = source.importpath_aliases,
        pathtype = source.pathtype,
        srcs = tuple(source.srcs),
        _cover = source.cover,
        _embedsrcs = tuple(source.embedsrcs),
        _x_defs = tuple(source.x_defs.items()),
        _gc_goopts = tuple(source.gc_goopts),
        _cgo = source.cgo,
        _cdeps = tuple(source.cdeps),
        _cppopts = tuple(source.cppopts),
        _copts = tuple(source.copts),
        _cxxopts = tuple(source.cxxopts),
        _clinkopts = tuple(source.clinkopts),

        # Information on dependencies
        _dep_labels = tuple([d.data.label for d in direct]),

        # Information needed by dependents
        file = out_lib,
        export_file = out_export,
        facts_file = out_facts,
        runfiles = source.runfiles,
        _validation_output = out_nogo_validation,
        _nogo_fix_output = out_nogo_fix,
        _cgo_deps = cgo_deps,
    )
    x_defs = dict(source.x_defs)
    for a in direct:
        x_defs.update(a.x_defs)

    # Ensure that the _cgo_export.h of the current target comes first when cgo_exports is iterated
    # by prepending it and specifying the order explicitly. This is required as the CcInfo attached
    # to the archive only exposes a single header rather than combining all headers.
    cgo_exports_direct = [out_cgo_export_h] if out_cgo_export_h else []
    cgo_exports = depset(direct = cgo_exports_direct, transitive = [a.cgo_exports for a in direct], order = "preorder")
    return GoArchive(
        source = source,
        data = data,
        direct = direct,
        libs = depset(direct = [out_lib], transitive = [a.libs for a in direct]),
        transitive = depset([data], transitive = [a.transitive for a in direct]),
        x_defs = x_defs,
        cgo_deps = depset(transitive = [cgo_deps] + [a.cgo_deps for a in direct]),
        cgo_exports = cgo_exports,
        runfiles = runfiles,
    )
