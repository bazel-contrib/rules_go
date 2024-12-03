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

# The filtered inputs and dependencies needed to build a GoArchive
# This is a configuration specific provider.
# It has no transitive information.
# See go/providers.rst#GoInfo for full documentation.
GoInfo = provider()

# This compiled form of a package used in transitive dependencies.
# This is a configuration specific provider.
# See go/providers.rst#GoArchiveData for full documentation.
GoArchiveData = provider()

# The compiled form of GoInfo, with everything needed to link it into a binary.
# This is a configuration specific provider.
# See go/providers.rst#GoArchive for full documentation.
GoArchive = provider()

GoPath = provider()

GoSDK = provider(
    doc = "Contains information about the Go SDK used in the toolchain",
    fields = {
        "goos": "The host OS the SDK was built for.",
        "goarch": "The host architecture the SDK was built for.",
        "experiments": "Comma-separated Go experiments to enable via GOEXPERIMENT.",
        "root_file": "A file in the SDK root directory",
        "libs": ("Depset of pre-compiled .a files for the standard library " +
                 "built for the execution platform."),
        "headers": ("Depset of .h files from pkg/include that may be included " +
                    "in assembly sources."),
        "srcs": ("Depset of source files for importable packages in the " +
                 "standard library. Internal, vendored, and tool packages " +
                 "may not be included."),
        "package_list": ("A file containing a list of importable packages " +
                         "in the standard library."),
        "tools": ("Depset of executable files in the SDK built for " +
                  "the execution platform, excluding the go binary file"),
        "go": "The go binary file",
        "version": "The Go SDK version",
    },
)

GoStdLib = provider()

GoConfigInfo = provider()

GoContextInfo = provider()

CgoContextInfo = provider()

EXPLICIT_PATH = "explicit"

INFERRED_PATH = "inferred"

EXPORT_PATH = "export"

def get_source(dep):
    if type(dep) == "struct":
        return dep
    return dep[GoInfo]

def get_archive(dep):
    if type(dep) == "struct":
        return dep
    return dep[GoArchive]

def effective_importpath_pkgpath(lib):
    """Returns import and package paths for a given lib with modifications for display.

    This is used when we need to represent sources in a manner compatible with Go
    build (e.g., for packaging or coverage data listing). _test suffixes are
    removed, and vendor directories from importmap may be modified.

    Args:
      lib: GoInfo or GoArchiveData

    Returns:
      A tuple of effective import path and effective package path. Both are ""
      for synthetic archives (e.g., generated testmain).
    """
    if lib.pathtype not in (EXPLICIT_PATH, EXPORT_PATH):
        return "", ""
    importpath = lib.importpath
    importmap = lib.importmap
    if importpath.endswith("_test"):
        importpath = importpath[:-len("_test")]
    if importmap.endswith("_test"):
        importmap = importmap[:-len("_test")]
    parts = importmap.split("/")
    if "vendor" not in parts:
        # Unusual case not handled by go build. Just return importpath.
        return importpath, importpath
    elif len(parts) > 2 and lib.label.workspace_root == "external/" + parts[0]:
        # Common case for importmap set by Gazelle in external repos.
        return importpath, importmap[len(parts[0]):]
    else:
        # Vendor directory somewhere in the main repo. Leave it alone.
        return importpath, importmap
