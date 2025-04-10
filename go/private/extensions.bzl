# Copyright 2023 The Bazel Authors. All rights reserved.
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

load("@io_bazel_rules_go_bazel_features//:features.bzl", "bazel_features")
load("//go/private:nogo.bzl", "DEFAULT_NOGO", "NOGO_DEFAULT_EXCLUDES", "NOGO_DEFAULT_INCLUDES", "go_register_nogo")
load("//go/private:sdk.bzl", "detect_host_platform", "go_download_sdk_rule", "go_host_sdk_rule", "go_multiple_toolchains", "go_wrap_sdk_rule")

def host_compatible_toolchain_impl(ctx):
    ctx.file("BUILD.bazel")
    ctx.file("defs.bzl", content = """
HOST_COMPATIBLE_SDK = Label({})
""".format(repr(ctx.attr.toolchain)))

host_compatible_toolchain = repository_rule(
    implementation = host_compatible_toolchain_impl,
    attrs = {
        # We cannot use attr.label for the `toolchain` attribute since the module extension cannot
        # refer to the repositories it creates by their apparent repository names.
        "toolchain": attr.string(
            doc = "The apparent label of a `ROOT` file in the repository of a host compatible toolchain created by the `go_sdk` extension",
            mandatory = True,
        ),
    },
    doc = "An external repository to expose the first host compatible toolchain",
)

_download_tag = tag_class(
    attrs = {
        "name": attr.string(),
        "goos": attr.string(),
        "goarch": attr.string(),
        "sdks": attr.string_list_dict(),
        "experiments": attr.string_list(
            doc = "Go experiments to enable via GOEXPERIMENT",
        ),
        "urls": attr.string_list(default = ["https://dl.google.com/go/{}"]),
        "version": attr.string(),
        "patches": attr.label_list(
            doc = "A list of patches to apply to the SDK after downloading it",
        ),
        "patch_strip": attr.int(
            default = 0,
            doc = "The number of leading path segments to be stripped from the file name in the patches.",
        ),
        "strip_prefix": attr.string(default = "go"),
    },
)

_host_tag = tag_class(
    attrs = {
        "name": attr.string(),
        "version": attr.string(),
        "experiments": attr.string_list(
            doc = "Go experiments to enable via GOEXPERIMENT",
        ),
    },
)

_nogo_tag = tag_class(
    attrs = {
        "nogo": attr.label(
            doc = "The nogo target to use when this module is the root module.",
        ),
        "includes": attr.label_list(
            default = NOGO_DEFAULT_INCLUDES,
            # The special include "all" is undocumented on purpose: With it, adding a new transitive
            # dependency to a Go module can cause a build failure if the new dependency has lint
            # issues.
            doc = """
A Go target is checked with nogo if its package matches at least one of the entries in 'includes'
and none of the entries in 'excludes'. By default, nogo is applied to all targets in the main
repository.

Uses the same format as 'visibility', i.e., every entry must be a label that ends with ':__pkg__' or
':__subpackages__'.
""",
        ),
        "excludes": attr.label_list(
            default = NOGO_DEFAULT_EXCLUDES,
            doc = "See 'includes'.",
        ),
    },
)

_wrap_tag = tag_class(
    attrs = {
        "root_file": attr.label(
            mandatory = False,
            doc = "A file in the SDK root directory. Use to determine GOROOT.",
        ),
        "root_files": attr.string_dict(
            mandatory = False,
            doc = "A set of mappings from the host platform to a file in the SDK's root directory.",
        ),
        "version": attr.string(),
        "experiments": attr.string_list(
            doc = "Go experiments to enable via GOEXPERIMENT.",
        ),
        "goos": attr.string(),
        "goarch": attr.string(),
    },
)

# A list of (goos, goarch) pairs that are commonly used for remote executors in cross-platform
# builds (where host != exec platform). By default, we register toolchains for all of these
# platforms in addition to the host platform.
_COMMON_EXEC_PLATFORMS = [
    ("darwin", "amd64"),
    ("darwin", "arm64"),
    ("linux", "amd64"),
    ("linux", "arm64"),
    ("windows", "amd64"),
    ("windows", "arm64"),
]

# This limit can be increased essentially arbitrarily, but doing so will cause a rebuild of all
# targets using any of these toolchains due to the changed repository name.
_MAX_NUM_TOOLCHAINS = 9999
_TOOLCHAIN_INDEX_PAD_LENGTH = len(str(_MAX_NUM_TOOLCHAINS))

def _go_sdk_impl(ctx):
    nogo_tag = struct(
        nogo = DEFAULT_NOGO,
        includes = NOGO_DEFAULT_INCLUDES,
        excludes = NOGO_DEFAULT_EXCLUDES,
    )
    for module in ctx.modules:
        if not module.is_root or not module.tags.nogo:
            continue
        if len(module.tags.nogo) > 1:
            # Make use of the special formatting applied to tags by fail.
            fail(
                "go_sdk.nogo: only one tag can be specified per module, got:\n",
                *[t for p in zip(module.tags.nogo, len(module.tags.nogo) * ["\n"]) for t in p]
            )
        nogo_tag = module.tags.nogo[0]
        for scope in nogo_tag.includes + nogo_tag.excludes:
            # Validate that the scope references a valid, visible repository.
            # buildifier: disable=no-effect
            scope.repo_name
            if scope.name != "__pkg__" and scope.name != "__subpackages__":
                fail(
                    "go_sdk.nogo: all entries in includes and excludes must end with ':__pkg__' or ':__subpackages__', got '{}' in".format(scope.name),
                    nogo_tag,
                )
    go_register_nogo(
        name = "io_bazel_rules_nogo",
        nogo = str(nogo_tag.nogo),
        # Go through canonical label literals to avoid a dependency edge on the packages in the
        # scope.
        includes = [str(l) for l in nogo_tag.includes],
        excludes = [str(l) for l in nogo_tag.excludes],
    )

    multi_version_module = {}
    for module in ctx.modules:
        if module.name in multi_version_module:
            multi_version_module[module.name] = True
        else:
            multi_version_module[module.name] = False

    # We remember the first host compatible toolchain declared by the download and host tags.
    # The order follows bazel's iteration over modules (the toolchains declared by the root module are considered first).
    # We know that at least `go_default_sdk` (which is declared by the `rules_go` module itself) is host compatible.
    first_host_compatible_toolchain = None
    host_detected_goos, host_detected_goarch = detect_host_platform(ctx)
    toolchains = []
    for module in ctx.modules:
        # Apply wrapped toolchains first to override specific platforms from the
        # default toolchain or any downloads.
        for index, wrap_tag in enumerate(module.tags.wrap):
            name = _default_go_sdk_name(
                module = module,
                multi_version = multi_version_module[module.name],
                tag_type = "wrap",
                index = index,
            )
            go_wrap_sdk_rule(
                name = name,
                root_file = wrap_tag.root_file,
                root_files = wrap_tag.root_files,
                version = wrap_tag.version,
                experiments = wrap_tag.experiments,
            )
            toolchains.append(struct(
                goos = wrap_tag.goos,
                goarch = wrap_tag.goarch,
                sdk_repo = name,
                sdk_type = "remote",
                sdk_version = wrap_tag.version,
            ))

        for index, download_tag in enumerate(module.tags.download):
            # SDKs without an explicit version are fetched even when not selected by toolchain
            # resolution. This is acceptable if brought in by the root module, but transitive
            # dependencies should not slow down the build in this way.
            if not module.is_root and not download_tag.version:
                fail("go_sdk.download: version must be specified in non-root module " + module.name)

            # SDKs with an explicit name are at risk of colliding with those from other modules.
            # This is acceptable if brought in by the root module as the user is responsible for any
            # conflicts that arise. rules_go itself provides "go_default_sdk".
            # TODO: Now that Gazelle relies on the go_host_compatible_sdk_label repo, remove the
            #       special case for "go_default_sdk". Users should migrate to @rules_go//go.
            if (not module.is_root and not module.name == "rules_go") and download_tag.name:
                fail("go_sdk.download: name must not be specified in non-root module " + module.name)

            name = download_tag.name or _default_go_sdk_name(
                module = module,
                multi_version = multi_version_module[module.name],
                tag_type = "download",
                index = index,
            )

            # Keep in sync with the other call to go_download_sdk_rule below.
            go_download_sdk_rule(
                name = name,
                goos = download_tag.goos,
                goarch = download_tag.goarch,
                sdks = download_tag.sdks,
                experiments = download_tag.experiments,
                patches = download_tag.patches,
                patch_strip = download_tag.patch_strip,
                urls = download_tag.urls,
                version = download_tag.version,
                strip_prefix = download_tag.strip_prefix,
            )

            if (not download_tag.goos or download_tag.goos == host_detected_goos) and (not download_tag.goarch or download_tag.goarch == host_detected_goarch):
                first_host_compatible_toolchain = first_host_compatible_toolchain or "@{}//:ROOT".format(name)

            toolchains.append(struct(
                goos = download_tag.goos,
                goarch = download_tag.goarch,
                sdk_repo = name,
                sdk_type = "remote",
                sdk_version = download_tag.version,
            ))

            # Additionally register SDKs for all common execution platforms, but only if the user
            # specified a version to prevent eager fetches.
            if download_tag.version and not download_tag.goos and not download_tag.goarch:
                for goos, goarch in _COMMON_EXEC_PLATFORMS:
                    if goos == host_detected_goos and goarch == host_detected_goarch:
                        # We already added the host-compatible toolchain above.
                        continue

                    if download_tag.sdks and not "{}_{}".format(goos, goarch) in download_tag.sdks:
                        # The user supplied custom download links, but not for this tuple.
                        continue

                    default_name = _default_go_sdk_name(
                        module = module,
                        multi_version = multi_version_module[module.name],
                        tag_type = "download",
                        index = index,
                        suffix = "_{}_{}".format(goos, goarch),
                    )
                    go_download_sdk_rule(
                        name = default_name,
                        goos = goos,
                        goarch = goarch,
                        sdks = download_tag.sdks,
                        experiments = download_tag.experiments,
                        patches = download_tag.patches,
                        patch_strip = download_tag.patch_strip,
                        urls = download_tag.urls,
                        version = download_tag.version,
                        strip_prefix = download_tag.strip_prefix,
                    )

                    toolchains.append(struct(
                        goos = goos,
                        goarch = goarch,
                        sdk_repo = default_name,
                        sdk_type = "remote",
                        sdk_version = download_tag.version,
                    ))

        for index, host_tag in enumerate(module.tags.host):
            # Dependencies can rely on rules_go providing a default remote SDK. They can also
            # configure a specific version of the SDK to use. However, they should not add a
            # dependency on the host's Go SDK.
            if not module.is_root:
                fail("go_sdk.host: cannot be used in non-root module {}, consider using use_extension(..., dev_dependency = True)".format(module.name))

            name = host_tag.name or _default_go_sdk_name(
                module = module,
                multi_version = multi_version_module[module.name],
                tag_type = "host",
                index = index,
            )
            go_host_sdk_rule(
                name = name,
                version = host_tag.version,
                experiments = host_tag.experiments,
            )

            toolchains.append(struct(
                goos = "",
                goarch = "",
                sdk_repo = name,
                sdk_type = "host",
                sdk_version = host_tag.version,
            ))
            first_host_compatible_toolchain = first_host_compatible_toolchain or "@{}//:ROOT".format(name)

    host_compatible_toolchain(name = "go_host_compatible_sdk_label", toolchain = first_host_compatible_toolchain)
    if len(toolchains) > _MAX_NUM_TOOLCHAINS:
        fail("more than {} go_sdk tags are not supported".format(_MAX_NUM_TOOLCHAINS))

    # Toolchains in a BUILD file are registered in the order given by name, not in the order they
    # are declared:
    # https://cs.opensource.google/bazel/bazel/+/master:src/main/java/com/google/devtools/build/lib/packages/Package.java;drc=8e41dce65b97a3d466d6b1e65005abc52a07b90b;l=156
    # We pad with an index that lexicographically sorts in the same order as if these toolchains
    # were registered using register_toolchains in their MODULE.bazel files.
    go_multiple_toolchains(
        name = "go_toolchains",
        prefixes = [
            _toolchain_prefix(index, toolchain.sdk_repo)
            for index, toolchain in enumerate(toolchains)
        ],
        geese = [toolchain.goos for toolchain in toolchains],
        goarchs = [toolchain.goarch for toolchain in toolchains],
        sdk_repos = [toolchain.sdk_repo for toolchain in toolchains],
        sdk_types = [toolchain.sdk_type for toolchain in toolchains],
        sdk_versions = [toolchain.sdk_version for toolchain in toolchains],
    )

    if bazel_features.external_deps.extension_metadata_has_reproducible:
        return ctx.extension_metadata(reproducible = True)
    else:
        return None

def _default_go_sdk_name(*, module, multi_version, tag_type, index, suffix = ""):
    # Keep the version and name of the root module out of the repository name if possible to
    # prevent unnecessary rebuilds when it changes.
    return "{name}_{version}_{tag_type}_{index}{suffix}".format(
        # "main_" is not a valid module name and thus can't collide.
        name = "main_" if module.is_root else module.name,
        version = module.version if multi_version else "",
        tag_type = tag_type,
        index = index,
        suffix = suffix,
    )

def _toolchain_prefix(index, name):
    """Prefixes the given name with the index, padded with zeros to ensure lexicographic sorting.

    Examples:
      _toolchain_prefix(   2, "foo") == "_0002_foo_"
      _toolchain_prefix(2000, "foo") == "_2000_foo_"
    """
    return "_{}_{}_".format(_left_pad_zero(index, _TOOLCHAIN_INDEX_PAD_LENGTH), name)

def _left_pad_zero(index, length):
    if index < 0:
        fail("index must be non-negative")
    return ("0" * length + str(index))[-length:]

go_sdk_extra_kwargs = {
    # The choice of a host-compatible SDK is expressed in repository rule attribute values and
    # depends on host OS and architecture.
    "os_dependent": True,
    "arch_dependent": True,
} if bazel_features.external_deps.module_extension_has_os_arch_dependent else {}

go_sdk = module_extension(
    implementation = _go_sdk_impl,
    tag_classes = {
        "download": _download_tag,
        "host": _host_tag,
        "nogo": _nogo_tag,
        "wrap": _wrap_tag,
    },
    **go_sdk_extra_kwargs
)
