load(
    "@bazel_skylib//lib:paths.bzl",
    "paths",
)

def write_pkg_json(ctx, pkg_json_tool, archive, pkg_json):
    args = ctx.actions.args()
    inputs = [src for src in archive.data.srcs if src.path.endswith(".go")]

    tmp_json = ctx.actions.declare_file(pkg_json.path + ".tmp")
    pkg_info = _go_archive_to_pkg(archive)
    ctx.actions.write(tmp_json, content = json.encode(pkg_info))
    inputs.append(tmp_json)
    args.add("--pkg_json", tmp_json.path)

    if archive.data.cgo_out_dir:
        inputs.append(archive.data.cgo_out_dir)
        args.add("--cgo_out_dir", file_path(archive.data.cgo_out_dir))

    args.add("--output", pkg_json.path)
    ctx.actions.run(
        inputs = inputs,
        outputs = [pkg_json],
        executable = pkg_json_tool.path,
        arguments = [args],
        tools = [pkg_json_tool],
    )

def file_path(f):
    prefix = "__BAZEL_WORKSPACE__"
    if not f.is_source:
        prefix = "__BAZEL_EXECROOT__"
    elif is_file_external(f):
        prefix = "__BAZEL_OUTPUT_BASE__"
    return paths.join(prefix, f.path)

def is_file_external(f):
    return f.owner.workspace_root != ""

def _go_archive_to_pkg(archive):
    go_files = [
        file_path(src)
        for src in archive.data.srcs
        if src.path.endswith(".go")
    ]
    return struct(
        ID = str(archive.data.label),
        PkgPath = archive.data.importpath,
        ExportFile = file_path(archive.data.export_file),
        GoFiles = go_files,
        CompiledGoFiles = go_files,
        OtherFiles = [
            file_path(src)
            for src in archive.data.srcs
            if not src.path.endswith(".go")
        ],
        Imports = {
            pkg.data.importpath: str(pkg.data.label)
            for pkg in archive.direct
        },
    )
