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
    "//go/private:providers.bzl",
    "GoSDK",
)

def _go_sdk_impl(ctx):
    package_list = ctx.file.package_list
    if package_list == None:
        package_list = ctx.actions.declare_file("packages.txt")
        _build_package_list(ctx, ctx.files.srcs, ctx.file.root_file, package_list)
    return [
        DefaultInfo(
            files = depset(
                [ctx.file.go] +
                ctx.files.libs +
                ctx.files.headers +
                ctx.files.srcs +
                ctx.files.tools,
            ),
        ),
        GoSDK(
            goos = ctx.attr.goos,
            goarch = ctx.attr.goarch,
            experiments = ",".join(ctx.attr.experiments),
            gofips140 = ctx.attr.gofips140,
            root_file = ctx.file.root_file,
            package_list = package_list,
            fips_package_list = ctx.file.fips_package_list,
            libs = depset(ctx.files.libs),
            headers = depset(ctx.files.headers),
            srcs = depset(ctx.files.srcs),
            tools = depset(ctx.files.tools),
            go = ctx.executable.go,
            version = ctx.attr.version,
        ),
    ]

go_sdk = rule(
    _go_sdk_impl,
    attrs = {
        "goos": attr.string(
            mandatory = True,
            doc = "The host OS the SDK was built for",
        ),
        "goarch": attr.string(
            mandatory = True,
            doc = "The host architecture the SDK was built for",
        ),
        "experiments": attr.string_list(
            mandatory = False,
            doc = "Go experiments to enable via GOEXPERIMENT",
        ),
        "gofips140": attr.string(
            default = "",
            doc = "GOFIPS140 version to build with (e.g. 'v1.0.0', 'latest', 'certified'). Empty string disables.",
        ),
        "root_file": attr.label(
            mandatory = True,
            allow_single_file = True,
            doc = "A file in the SDK root directory. Used to determine GOROOT.",
            cfg = "exec",
        ),
        "package_list": attr.label(
            allow_single_file = True,
            doc = ("A text file containing a list of packages in the " +
                   "standard library that may be imported."),
        ),
        "fips_package_list": attr.label(
            allow_single_file = True,
            doc = ("A text file listing the versioned GOFIPS140 snapshot " +
                   "packages the stdlib builder must place into pkg/. " +
                   "Empty/absent for non-FIPS SDKs."),
        ),
        "libs": attr.label_list(
            # allow_files is not set to [".a"] because that wouldn't allow
            # for zero files to be present, as is the case in Go 1.20+.
            # See also https://github.com/bazelbuild/bazel/issues/7516
            #
            # These pre-compiled .a files are semantically target-configured,
            # so unlike the other SDK layout attrs they are left unpinned. In
            # practice this is moot: since none are present the bootstrap SDK
            # never produces them under a mismatched configuration.
            allow_files = True,
            doc = ("Pre-compiled .a files for the standard library, " +
                   "built for the execution platform"),
        ),
        "headers": attr.label_list(
            cfg = "exec",
            allow_files = [".h"],
            doc = (".h files from pkg/include that may be included in " +
                   "assembly sources"),
        ),
        "srcs": attr.label_list(
            cfg = "exec",
            allow_files = True,
            doc = "Source files for packages in the standard library",
        ),
        "tools": attr.label_list(
            allow_files = True,
            cfg = "exec",
            doc = ("List of executable files in the SDK built for " +
                   "the execution platform, excluding the go binary"),
        ),
        "go": attr.label(
            mandatory = True,
            allow_single_file = True,
            executable = True,
            cfg = "exec",
            doc = "The go binary",
        ),
        "version": attr.string(
            doc = "The version of the Go SDK.",
        ),
    },
    doc = ("Collects information about a Go SDK. The SDK must have a normal " +
           "GOROOT directory structure."),
    provides = [GoSDK],
)

def _package_list_impl(ctx):
    out = ctx.outputs.out
    fips_out = ctx.outputs.fips_out

    # Normal standard-library package list, derived from the source tree layout
    # (package File paths, which are available during analysis).
    normal = _package_names(ctx.files.srcs, ctx.file.root_file)

    # Versioned GOFIPS140 snapshot packages. The snapshot zip under lib/fips140
    # cannot be read during analysis, so it is enumerated at SDK-fetch time and
    # baked into this attribute (see _fips_snapshot_packages in go/private/sdk.bzl).
    # That lets both outputs be produced with plain ctx.actions.write: no shell
    # action, no external tool, and no dependency on the SDK itself.
    fips = ctx.attr.fips_packages

    # fips_out: the versioned FIPS packages only, read verbatim by the stdlib
    # builder (installFIPSSnapshotArchives). Empty for non-FIPS SDKs.
    ctx.actions.write(fips_out, _lines(sorted(fips)))

    # out (packages.txt): normal stdlib list plus the FIPS packages, consumed by
    # the linker's importcfg.
    combined = dict(normal)
    for pkg in fips:
        combined[pkg] = None
    ctx.actions.write(out, _lines(sorted(combined.keys())))

    return [DefaultInfo(files = depset([out]))]

package_list = rule(
    _package_list_impl,
    attrs = {
        "srcs": attr.label_list(
            allow_files = True,
            doc = "Source files for packages in the standard library",
        ),
        "root_file": attr.label(
            mandatory = True,
            allow_single_file = True,
            doc = "A file in the SDK root directory. Used to determine GOROOT.",
        ),
        "fips_packages": attr.string_list(
            doc = "Versioned GOFIPS140 snapshot import paths (e.g. " +
                  "crypto/internal/fips140/<ver>/aes), enumerated from the " +
                  "snapshot zip at SDK-fetch time. Empty for non-FIPS SDKs.",
        ),
        "out": attr.output(
            mandatory = True,
            doc = "File to write. Must be 'packages.txt'.",
            # Gazelle depends on this file directly. It has to be an output
            # attribute because Bazel has no other way of knowing what rule
            # produces this file.
            # TODO(jayconrod): Update Gazelle and simplify this.
        ),
        "fips_out": attr.output(
            mandatory = True,
            doc = "File to write listing the versioned GOFIPS140 snapshot " +
                  "packages (empty for non-FIPS SDKs). Read by the stdlib builder.",
        ),
    },
)

def _lines(items):
    return "".join([item + "\n" for item in items])

def _package_names(srcs, root_file):
    packages = {}
    src_dir = root_file.dirname + "/src/"
    for src in srcs:
        pkg_src_dir = src.dirname
        if not pkg_src_dir.startswith(src_dir):
            continue
        packages[pkg_src_dir[len(src_dir):]] = None
    return packages

def _build_package_list(ctx, srcs, root_file, out):
    ctx.actions.write(out, _lines(sorted(_package_names(srcs, root_file).keys())))
