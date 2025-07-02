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

# TODO: Replace with a Bazel-provided function when it's available.
# https://github.com/bazelbuild/bazel/issues/14307
def _rlocation_path(ctx, file):
    """Returns the path relative to the runfiles directory."""
    if file.short_path.startswith("../"):
        return file.short_path[3:]
    else:
        return ctx.workspace_name + "/" + file.short_path

def _rpath(go, library, executable = None):
    """Returns the potential rpaths of a library, possibly relative to another file."""
    if not executable:
        return [paths.dirname(library.short_path)]

    origin = go.mode.goos == "darwin" and "@loader_path" or "$ORIGIN"

    # Accomodate for two kinds of executable paths.
    rpaths = []

    # 1. Where the executable is inside its own .runfiles directory.
    #  This is the case for generated libraries as well as remote builds.
    #   a) go back to the runfiles root from the executable file in .runfiles
    depth = _rlocation_path(go._ctx, executable).count("/")
    back_to_root = paths.join(*([".."] * depth))

    #   b) then walk back to the library's dir within runfiles dir.
    rpaths.append(paths.join(origin, back_to_root, paths.dirname(_rlocation_path(go._ctx, library))))

    # 2. Where the executable is outside the .runfiles directory:
    #  This is the case for local pre-built libraries, as well as local
    #  generated libraries.
    runfiles_dir = paths.basename(executable.short_path) + ".runfiles"
    rpaths.append(paths.join(origin, runfiles_dir, go._ctx.workspace_name, paths.dirname(library.short_path)))

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
