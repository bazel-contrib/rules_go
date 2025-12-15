# Copyright 2021 The Bazel Authors. All rights reserved.
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

def _rpath(go, library, executable = None):
    """Returns the potential rpaths of a library, possibly relative to another file."""
    if not executable:
        return [paths.dirname(library.short_path)]

    origin = "@loader_path" if go.mode.goos == "darwin" else "$ORIGIN"

    # Accommodate for three kinds of executable paths.
    rpaths = []
    library_dir = paths.dirname(library.short_path)

    # Based on the logic for Bazel's own C++ rules:
    # https://github.com/bazelbuild/bazel/blob/51a4b8e5de225ba163d19ddcc330aff8860a1520/src/main/starlark/builtins_bzl/common/cc/link/collect_solib_dirs.bzl#L301
    # with the bug fix https://github.com/bazelbuild/bazel/pull/27154.
    # We ignore the cases for --experimental_sibling_repository_layout.

    # 1. Where the executable is inside its own .runfiles directory.
    # This covers the cases 1, 3, 4, 5, and 7 in the linked code above.
    #   a) go back to the workspace root from the executable file in .runfiles
    depth = executable.short_path.count("/")
    back_to_root = paths.join(*([".."] * depth))

    #   b) then walk back to the library's short path
    rpaths.append(paths.join(origin, back_to_root, library_dir))

    # 2. Where the executable is outside the .runfiles directory:
    # This covers the cases 2 and 6 in the linked code above.
    runfiles_dir = paths.basename(executable.short_path) + ".runfiles"
    rpaths.append(paths.join(origin, runfiles_dir, go._ctx.workspace_name, library_dir))

    # 3. Where the executable is from a different repo
    # This covers the case 8 in the linked code above.
    if executable.short_path.startswith("../"):
        back_to_repo_root = paths.join(*([".."] * (depth - 1)))
        rpaths.append(paths.join(origin, back_to_repo_root, go._ctx.workspace_name, library_dir))

    return rpaths

def _flags(go, *args, **kwargs):
    """Returns the rpath linker flags for a library."""
    return ["-Wl,-rpath," + p for p in _rpath(go, *args, **kwargs)]

def _install_name(f):
    """Returns the install name for a dylib on macOS."""
    return f.short_path

rpath = struct(
    flags = _flags,
    install_name = _install_name,
    rpath = _rpath,
)
