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

GO_TOOLCHAIN = "@io_bazel_rules_go//go:toolchain"
GO_TOOLCHAIN_LABEL = Label(GO_TOOLCHAIN)

go_exts = [
    ".go",
]

asm_exts = [
    ".s",
    ".S",
    ".h",  # may be included by .s
]

# be consistent to cc_library.
hdr_exts = [
    ".h",
    ".hh",
    ".hpp",
    ".hxx",
    ".inc",
]

c_exts = [
    ".c",
    ".h",
]

cxx_exts = [
    ".cc",
    ".cxx",
    ".cpp",
    ".h",
    ".hh",
    ".hpp",
    ".hxx",
]

objc_exts = [
    ".m",
    ".mm",
    ".h",
    ".hh",
    ".hpp",
    ".hxx",
]

cgo_exts = [
    ".c",
    ".cc",
    ".cpp",
    ".cxx",
    ".h",
    ".hh",
    ".hpp",
    ".hxx",
    ".inc",
    ".m",
    ".mm",
]

syso_exts = [
    ".syso",
]

def os_path(ctx, path):
    path = str(path)  # maybe convert from path type
    if ctx.os.name.startswith("windows"):
        path = path.replace("/", "\\")
    return path

def executable_path(ctx, path):
    path = os_path(ctx, path)
    if ctx.os.name.startswith("windows"):
        path += ".exe"
    return path

def executable_extension(ctx):
    extension = ""
    if ctx.os.name.startswith("windows"):
        extension = ".exe"
    return extension

def goos_to_extension(goos):
    if goos == "windows":
        return ".exe"
    return ""

ARCHIVE_EXTENSION = ".a"

SHARED_LIB_EXTENSIONS = [".dll", ".dylib", ".so", ".wasm"]

def goos_to_shared_extension(goos):
    return {
        "windows": ".dll",
        "darwin": ".dylib",
        "wasip1": ".wasm",
    }.get(goos, ".so")

def has_shared_lib_extension(path):
    """
    Matches filenames of shared libraries, with or without a version number extension.
    """
    return (has_simple_shared_lib_extension(path) or
            get_versioned_shared_lib_extension(path))

def has_simple_shared_lib_extension(path):
    """
    Matches filenames of shared libraries, without a version number extension.
    """
    return any([path.endswith(ext) for ext in SHARED_LIB_EXTENSIONS])

def is_valid_shared_lib_version_part(part):
    """
    Checks if the version part is valid. Matches the same check from
    //src/main/java/com/google/devtools/build/lib/rules/cpp:CppFileTypes.java
    """
    if not part[:1].isdigit():
        return False
    for c in part[1:].elems():
        if c != "_" and not c.isalnum():
            return False
    return True

def get_versioned_shared_lib_extension(path):
    """If appears to be an versioned .so or .dylib file, return the extension; otherwise empty"""
    parts = path.split("/")[-1].split(".")
    if not is_valid_shared_lib_version_part(parts[-1]):
        return ""

    # only iterating to 1 because parts[0] has to be the lib name
    for i in range(len(parts) - 1, 0, -1):
        if not is_valid_shared_lib_version_part(parts[i]):
            if parts[i] == "dylib" or parts[i] == "so":
                return ".".join(parts[i:])

            # something like foo.bar.1.2 or dylib.1.2
            return ""

    # something like 1.2.3, or so.1.2, or dylib.1.2, or foo.1.2
    return ""

MINIMUM_BAZEL_VERSION = "6.5.0"

def as_iterable(v):
    """Returns a list, tuple, or depset as something iterable."""
    if type(v) == "list":
        return v
    if type(v) == "tuple":
        return v
    if type(v) == "depset":
        return v.to_list()
    fail("as_iterator failed on {}".format(v))

def count_group_matches(v, prefix, suffix):
    """Counts reluctant substring matches between prefix and suffix.

    Equivalent to the number of regular expression matches "prefix.+?suffix"
    in the string v.
    """

    count = 0
    idx = 0
    for i in range(0, len(v)):
        if idx > i:
            continue

        idx = v.find(prefix, idx)
        if idx == -1:
            break

        # If there is another prefix before the next suffix, the previous prefix is discarded.
        # This is OK; it does not affect our count.
        idx = v.find(suffix, idx)
        if idx == -1:
            break

        count = count + 1

    return count

# C/C++ compiler and linker options related to coverage instrumentation.
COVERAGE_OPTIONS_DENYLIST = {
    "--coverage": None,
    "-ftest-coverage": None,
    "-fprofile-arcs": None,
    "-fprofile-instr-generate": None,
    "-fcoverage-mapping": None,
}

RULES_GO_REPO_NAME = str(Label("//:unused"))[:-len("//:unused")]
RULES_GO_STDLIB_PREFIX = RULES_GO_REPO_NAME + "//stdlib:"

# TODO: Remove the "and" once the rules_go repo itself uses Bzlmod.
RULES_GO_IS_BZLMOD_REPO = RULES_GO_REPO_NAME.lstrip("@") != "io_bazel_rules_go" and RULES_GO_REPO_NAME.lstrip("@")

# Marks an action as supporting path mapping (--experimental_output_paths=strip).
# See https://www.youtube.com/watch?v=Et1rjb7ixUU for more details.
SUPPORTS_PATH_MAPPING_REQUIREMENT = {"supports-path-mapping": "1"}
