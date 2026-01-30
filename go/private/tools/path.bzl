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
    "@bazel_skylib//lib:paths.bzl",
    "paths",
)
load(
    "//go/private:providers.bzl",
    "GoArchive",
    "GoPath",
    "effective_importpath_pkgpath",
)

def _go_path_impl(ctx):
    # Gather all archives. Note that there may be multiple packages with the same
    # importpath (e.g., multiple vendored libraries, internal tests). The same
    # package may also appear in different modes.
    mode_to_deps = {}
    for dep in ctx.attr.deps:
        archive = dep[GoArchive]
        mode = archive.source.mode
        if mode not in mode_to_deps:
            mode_to_deps[mode] = []
        mode_to_deps[mode].append(archive)
    mode_to_archive = {}
    for mode, archives in mode_to_deps.items():
        direct = [a.data for a in archives]
        transitive = []
        if ctx.attr.include_transitive:
            transitive = [a.transitive for a in archives]
        mode_to_archive[mode] = depset(direct = direct, transitive = transitive)

    # Collect sources and data files from archives. Merge archives into packages.
    pkg_map = {}  # map from package path to structs
    for mode, archives in mode_to_archive.items():
        for archive in archives.to_list():
            importpath, pkgpath = effective_importpath_pkgpath(archive)
            if importpath == "":
                continue  # synthetic archive or inferred location
            pkg = struct(
                importpath = importpath,
                dir = "src/" + pkgpath,
                srcs = list(archive.srcs),
                runfiles = archive.runfiles,
                embedsrcs = list(archive._embedsrcs),
                pkgs = {mode: archive.file},
            )
            if pkgpath in pkg_map:
                pkg = _merge_pkg(pkg_map[pkgpath], pkg)
            pkg_map[pkgpath] = pkg

    # Build a manifest file that includes all files to copy/link/zip.
    inputs = []
    manifest_entries = []
    manifest_entry_map = {}
    for pkg in pkg_map.values():
        # src_dir is the path to the directory holding the source.
        # Paths to embedded sources will be relative to this path.
        src_dir = None

        for f in pkg.srcs:
            src_dir = f.dirname
            dst = pkg.dir + "/" + f.basename
            _add_manifest_entry(manifest_entries, manifest_entry_map, inputs, f, dst)
        for f in pkg.embedsrcs:
            if src_dir == None:
                fail("cannot relativize {}: src_dir is unset".format(f.path))
            embedpath = paths.relativize(f.path, f.root.path)
            dst = pkg.dir + "/" + paths.relativize(embedpath.lstrip(ctx.bin_dir.path + "/"), src_dir.lstrip(ctx.bin_dir.path + "/"))
            _add_manifest_entry(manifest_entries, manifest_entry_map, inputs, f, dst)
    if ctx.attr.include_pkg:
        for pkg in pkg_map.values():
            for mode, f in pkg.pkgs.items():
                # TODO(jayconrod): include other mode attributes, e.g., race.
                installsuffix = mode.goos + "_" + mode.goarch
                dst = "pkg/" + installsuffix + "/" + pkg.dir[len("src/"):] + ".a"
                _add_manifest_entry(manifest_entries, manifest_entry_map, inputs, f, dst)
    if ctx.attr.include_data:
        for pkg in pkg_map.values():
            for f in pkg.runfiles.files.to_list():
                parts = f.path.split("/")
                if "testdata" in parts:
                    i = parts.index("testdata")
                    dst = pkg.dir + "/" + "/".join(parts[i:])
                else:
                    dst = pkg.dir + "/" + f.basename
                _add_manifest_entry(manifest_entries, manifest_entry_map, inputs, f, dst)
    for f in ctx.files.data:
        _add_manifest_entry(
            manifest_entries,
            manifest_entry_map,
            inputs,
            f,
            f.basename,
        )
    manifest_file = ctx.actions.declare_file(ctx.label.name + "~manifest")
    manifest_entries_json = [json.encode(e) for e in manifest_entries]
    manifest_content = "[\n  " + ",\n  ".join(manifest_entries_json) + "\n]"
    ctx.actions.write(manifest_file, manifest_content)
    inputs.append(manifest_file)

    # Execute the builder
    if ctx.attr.mode == "archive":
        out = ctx.actions.declare_file(ctx.label.name + ".zip")
        out_path = out.path
        out_short_path = out.short_path
        outputs = [out]
        out_file = out
    elif ctx.attr.mode == "copy":
        out = ctx.actions.declare_directory(ctx.label.name)
        out_path = out.path
        out_short_path = out.short_path
        outputs = [out]
        out_file = out
    else:  # link
        # Declare individual outputs in link mode. Symlinks can't point outside
        # tree artifacts.
        outputs = [
            ctx.actions.declare_file(ctx.label.name + "/" + e.dst)
            for e in manifest_entries
        ]
        tag = ctx.actions.declare_file(ctx.label.name + "/.tag")
        ctx.actions.write(tag, "")
        out_path = tag.dirname
        out_short_path = tag.short_path.rpartition("/")[0]
        out_file = tag
    args = ctx.actions.args()
    args.add("-manifest", manifest_file)
    args.add("-out", out_path)
    args.add("-mode", ctx.attr.mode)
    ctx.actions.run(
        outputs = outputs,
        inputs = inputs,
        mnemonic = "GoPath",
        executable = ctx.executable._go_path,
        arguments = [args],
    )

    return [
        DefaultInfo(
            files = depset(outputs),
            runfiles = ctx.runfiles(files = outputs),
        ),
        GoPath(
            gopath = out_short_path,
            gopath_file = out_file,
            packages = pkg_map.values(),
        ),
    ]

go_path = rule(
    _go_path_impl,
    attrs = {
        "deps": attr.label_list(
            providers = [GoArchive],
            doc = """A list of targets that build Go packages. A directory will be generated from
            files in these targets and their transitive dependencies. All targets must
            provide [GoArchive] ([go_library], [go_binary], [go_test], and similar
            rules have this).

            Only targets with explicit `importpath` attributes will be included in the
            generated directory. Synthetic packages (like the main package produced by
            [go_test]) and packages with inferred import paths will not be
            included. The values of `importmap` attributes may influence the placement
            of packages within the generated directory (for example, in vendor
            directories).

            The generated directory will contain original source files, including .go,
            .s, .h, and .c files compiled by cgo. It will not contain files generated by
            tools like cover and cgo, but it will contain generated files passed in
            `srcs` attributes like .pb.go files. The generated directory will also
            contain runfiles found in `data` attributes.
            """,
        ),
        "data": attr.label_list(
            allow_files = True,
            doc = """
            A list of targets producing data files that will be stored next to the
            `src/` directory. Useful for including things like licenses and readmes.
            """,
        ),
        "mode": attr.string(
            default = "copy",
            values = [
                "archive",
                "copy",
                "link",
            ],
            doc = """
            Determines how the generated directory is provided. May be one of:
            <ul>
                <li>`"archive"`: The generated directory is packaged as a single .zip file.</li>
                <li>`"copy"`: The generated directory is a single tree artifact. Source files
                are copied into the tree.</li>
                <li>`"link"`: **Unmaintained due to correctness issues**. Source files
                are symlinked into the tree. All of the symlink files are provided as separate output
                files.</li>
            </ul>

            ***Note:*** In `"copy"` mode, when a `GoPath` is consumed as a set of input
            files or run files, Bazel may provide symbolic links instead of regular files.
            Any program that consumes these files should dereference links, e.g., if you
            run `tar`, use the `--dereference` flag.
            """,
        ),
        "include_data": attr.bool(
            default = True,
            doc = """
            When true, data files referenced by libraries, binaries, and tests will be
            included in the output directory. Files listed in the `data` attribute
            for this rule will be included regardless of this attribute.
            """,
        ),
        "include_pkg": attr.bool(
            default = False,
            doc = """
            When true, a `pkg` subdirectory containing the compiled libraries will be created in the
            generated `GOPATH` containing compiled libraries.
            """,
        ),
        "include_transitive": attr.bool(
            default = True,
            doc = """
            When true, the transitive dependency graph will be included in the generated `GOPATH`. This is
            the default behaviour. When false, only the direct dependencies will be included in the
            generated `GOPATH`.
            """,
        ),
        "_go_path": attr.label(
            default = "//go/tools/builders:go_path",
            executable = True,
            cfg = "exec",
        ),
    },
    doc = """`go_path` builds a directory structure that can be used with
    tools that understand the GOPATH directory layout. This directory structure
    can be built by zipping, copying, or linking files.
    `go_path` can depend on one or more Go targets (i.e., [go_library], [go_binary], or [go_test]).
    It will include packages from those targets, as well as their transitive dependencies.
    Packages will be in subdirectories named after their `importpath` or `importmap` attributes under a `src/` directory.
    """,
)

def _merge_pkg(x, y):
    x_srcs = {f.path: None for f in x.srcs}
    x_embedsrcs = {f.path: None for f in x.embedsrcs}

    # Not all bazel versions support `dict1 | dict2` yet.
    pkgs = dict()
    pkgs.update(x.pkgs)
    pkgs.update(y.pkgs)

    return struct(
        importpath = x.importpath,
        dir = x.dir,
        srcs = x.srcs + [f for f in y.srcs if f.path not in x_srcs],
        runfiles = x.runfiles.merge(y.runfiles),
        embedsrcs = x.embedsrcs + [f for f in y.embedsrcs if f.path not in x_embedsrcs],
        pkgs = pkgs,
    )

def _add_manifest_entry(entries, entry_map, inputs, src, dst):
    if dst in entry_map:
        if entry_map[dst] != src.path:
            fail("{}: references multiple files ({} and {})".format(dst, entry_map[dst], src.path))
        return
    entries.append(struct(src = src.path, dst = dst))
    entry_map[dst] = src.path
    inputs.append(src)
