load(
    "@bazel_skylib//lib:paths.bzl",
    "paths",
)

def write_pkg_json(ctx, pkg_json_tool, archive, output_json):
    inputs = [src for src in archive.data.srcs if src.path.endswith(".go")]
    input_paths = [file_path(src) for src in inputs]
    cgo_out_dir = ""
    if archive.data.cgo_out_dir:
        inputs.append(archive.data.cgo_out_dir)
        cgo_out_dir = file_path(archive.data.cgo_out_dir)

    ctx.actions.run(
        inputs = inputs,
        outputs = [output_json],
        executable = pkg_json_tool,
        arguments = [
            "--id",
            str(archive.data.label),
            "--pkg_path",
            archive.data.importpath,
            "--export_file",
            file_path(archive.data.export_file),
            "--go_files",
            ",".join(input_paths),
            "--compiled_go_files",
            ",".join(input_paths),
            "--cgo_out_dir",
            cgo_out_dir,
            "--other_files",
            ",".join([file_path(src) for src in archive.data.srcs if not src.path.endswith(".go")]),
            "--imports",
            ",".join([pkg.data.importpath + "=" + str(pkg.data.label) for pkg in archive.direct]),
            "--output",
            output_json.path,
        ],
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
