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

load("@bazel_tools//tools/build_defs/repo:utils.bzl", "patch", "read_user_netrc", "use_netrc")
load("//go/private:common.bzl", "executable_path")
load("//go/private:nogo.bzl", "go_register_nogo")
load("//go/private:platforms.bzl", "GOARCH_CONSTRAINTS", "GOOS_CONSTRAINTS")

MIN_SUPPORTED_VERSION = (1, 14, 0)

def _go_host_sdk_impl(ctx):
    goroot = _detect_host_sdk(ctx)
    platform = _detect_sdk_platform(ctx, goroot)
    version = _detect_sdk_version(ctx, goroot)
    _sdk_build_file(ctx, platform, version, experiments = ctx.attr.experiments, toolchain_experiments = ctx.attr.toolchain_experiments)
    _local_sdk(ctx, goroot)

go_host_sdk_rule = repository_rule(
    implementation = _go_host_sdk_impl,
    environ = ["GOROOT"],
    attrs = {
        "version": attr.string(),
        "experiments": attr.string_list(
            doc = "Go experiments to enable via GOEXPERIMENT",
        ),
        "toolchain_experiments": attr.string_list(
            doc = "Go experiments to enable via GOEXPERIMENT for toolchain tools",
        ),
        "_sdk_build_file": attr.label(
            default = Label("//go/private:BUILD.sdk.bazel"),
        ),
    },
)

def go_host_sdk(name, register_toolchains = True, **kwargs):
    go_host_sdk_rule(name = name, **kwargs)
    _go_toolchains(
        name = name + "_toolchains",
        sdk_repo = name,
        sdk_type = "host",
        sdk_version = kwargs.get("version"),
        goos = kwargs.get("goos"),
        goarch = kwargs.get("goarch"),
    )
    if register_toolchains:
        _register_toolchains(name)

def _go_download_sdk_impl(ctx):
    if not ctx.attr.goos and not ctx.attr.goarch:
        goos, goarch = detect_host_platform(ctx)
    else:
        if not ctx.attr.goos:
            fail("goarch set but goos not set")
        if not ctx.attr.goarch:
            fail("goos set but goarch not set")
        goos, goarch = ctx.attr.goos, ctx.attr.goarch
    platform = goos + "_" + goarch

    version = ctx.attr.version
    sdks = ctx.attr.sdks

    if not version:
        if ctx.attr.patches:
            fail("a single version must be specified to apply patches")

    if not sdks:
        # If sdks was unspecified, download a full list of files.
        # If version was unspecified, pick the latest version.
        # Even if version was specified, we need to download the file list
        # to find the SHA-256 sum. If we don't have it, Bazel won't cache
        # the downloaded archive.
        if not version:
            ctx.report_progress("Finding latest Go version")
        else:
            ctx.report_progress("Finding Go SHA-256 sums")
        sdks_by_version = fetch_sdks_by_version(ctx)

        if not version:
            highest_version = None
            for v in sdks_by_version.keys():
                pv = parse_version(v)
                if not pv or _version_is_prerelease(pv):
                    # skip parse errors and pre-release versions
                    continue
                if not highest_version or _version_less(highest_version, pv):
                    highest_version = pv
            if not highest_version:
                fail("did not find any Go versions in https://go.dev/dl/?mode=json")
            version = _version_string(highest_version)
        if version not in sdks_by_version:
            fail("did not find version {} in https://go.dev/dl/?mode=json".format(version))
        sdks = sdks_by_version[version]

    if platform not in sdks:
        fail("unsupported platform {}".format(platform))
    filename, sha256 = sdks[platform]

    _remote_sdk(ctx, [url.format(filename) for url in ctx.attr.urls], ctx.attr.strip_prefix, sha256)
    patch(ctx, patch_args = _get_patch_args(ctx.attr.patch_strip))

    detected_version = _detect_sdk_version(ctx, ".")
    _sdk_build_file(ctx, platform, detected_version, experiments = ctx.attr.experiments, toolchain_experiments = ctx.attr.toolchain_experiments)

    if not ctx.attr.sdks and not ctx.attr.version:
        # Returning this makes Bazel print a message that 'version' must be
        # specified for a reproducible build.
        return {
            "name": ctx.attr.name,
            "goos": ctx.attr.goos,
            "goarch": ctx.attr.goarch,
            "sdks": ctx.attr.sdks,
            "urls": ctx.attr.urls,
            "version": version,
            "strip_prefix": ctx.attr.strip_prefix,
        }

    if hasattr(ctx, "repo_metadata"):
        return ctx.repo_metadata(reproducible = True)
    else:
        return None

go_download_sdk_rule = repository_rule(
    implementation = _go_download_sdk_impl,
    attrs = {
        "goos": attr.string(),
        "goarch": attr.string(),
        "sdks": attr.string_list_dict(),
        "experiments": attr.string_list(
            doc = "Go experiments to enable via GOEXPERIMENT",
        ),
        "toolchain_experiments": attr.string_list(
            doc = "Go experiments to enable via GOEXPERIMENT for toolchain tools",
        ),
        "urls": attr.string_list(default = ["https://dl.google.com/go/{}"]),
        "version": attr.string(),
        "strip_prefix": attr.string(default = "go"),
        "patches": attr.label_list(
            doc = "A list of patches to apply to the SDK after downloading it",
        ),
        "patch_strip": attr.int(
            default = 0,
            doc = "The number of leading path segments to be stripped from the file name in the patches.",
        ),
        "_sdk_build_file": attr.label(
            default = Label("//go/private:BUILD.sdk.bazel"),
        ),
    },
)

def _define_version_constants(version, prefix = ""):
    pv = parse_version(version)
    if pv == None or len(pv) < 3:
        fail("error parsing sdk version: " + version)
    major, minor, patch = pv[0], pv[1], pv[2]
    prerelease = pv[3] if len(pv) > 3 else ""
    return """
{prefix}MAJOR_VERSION = "{major}"
{prefix}MINOR_VERSION = "{minor}"
{prefix}PATCH_VERSION = "{patch}"
{prefix}PRERELEASE_SUFFIX = "{prerelease}"
""".format(
        prefix = prefix,
        major = major,
        minor = minor,
        patch = patch,
        prerelease = prerelease,
    )

def _to_constant_name(s):
    # Prefix with _ as identifiers are not allowed to start with numbers.
    return "_" + "".join([c if c.isalnum() else "_" for c in s.elems()]).upper()

def _get_patch_args(patch_strip):
    if patch_strip:
        return ["-p{}".format(patch_strip)]
    return []

def go_toolchains_single_definition(ctx, *, prefix, goos, goarch, sdk_repo, sdk_type, sdk_version):
    if not goos and not goarch:
        goos, goarch = detect_host_platform(ctx)
    else:
        if not goos:
            fail("goarch set but goos not set")
        if not goarch:
            fail("goos set but goarch not set")

    chunks = []
    loads = []
    identifier_prefix = _to_constant_name(prefix)

    # If a sdk_version attribute is provided, use that version. This avoids
    # eagerly fetching the SDK repository. But if it's not provided, we have
    # no choice and must load version constants from the version.bzl file that
    # _sdk_build_file creates. This will trigger an eager fetch.
    if sdk_version:
        chunks.append(_define_version_constants(sdk_version, prefix = identifier_prefix))
    else:
        loads.append("""load(
    "@{sdk_repo}//:version.bzl",
    {identifier_prefix}MAJOR_VERSION = "MAJOR_VERSION",
    {identifier_prefix}MINOR_VERSION = "MINOR_VERSION",
    {identifier_prefix}PATCH_VERSION = "PATCH_VERSION",
    {identifier_prefix}PRERELEASE_SUFFIX = "PRERELEASE_SUFFIX",
)
""".format(
            sdk_repo = sdk_repo,
            identifier_prefix = identifier_prefix,
        ))

    chunks.append("""declare_bazel_toolchains(
    prefix = "{prefix}",
    go_toolchain_repo = "@{sdk_repo}",
    exec_goarch = "{goarch}",
    exec_goos = "{goos}",
    major = {identifier_prefix}MAJOR_VERSION,
    minor = {identifier_prefix}MINOR_VERSION,
    patch = {identifier_prefix}PATCH_VERSION,
    prerelease = {identifier_prefix}PRERELEASE_SUFFIX,
    sdk_name = "{sdk_repo}",
    sdk_type = "{sdk_type}",
)
""".format(
        prefix = prefix,
        identifier_prefix = identifier_prefix,
        sdk_repo = sdk_repo,
        goarch = goarch,
        goos = goos,
        sdk_type = sdk_type,
    ))

    return struct(
        loads = loads,
        chunks = chunks,
    )

def go_toolchains_build_file_content(
        ctx,
        prefixes,
        geese,
        goarchs,
        sdk_repos,
        sdk_types,
        sdk_versions):
    if not _have_same_length(prefixes, geese, goarchs, sdk_repos, sdk_types, sdk_versions):
        fail("all lists must have the same length")

    loads = [
        """load("@io_bazel_rules_go//go/private:go_toolchain.bzl", "declare_bazel_toolchains")""",
    ]
    chunks = [
        """package(default_visibility = ["//visibility:public"])""",
    ]

    for i in range(len(geese)):
        definition = go_toolchains_single_definition(
            ctx,
            prefix = prefixes[i],
            goos = geese[i],
            goarch = goarchs[i],
            sdk_repo = sdk_repos[i],
            sdk_type = sdk_types[i],
            sdk_version = sdk_versions[i],
        )
        loads.extend(definition.loads)
        chunks.extend(definition.chunks)

    return "\n".join(loads + chunks)

def _go_multiple_toolchains_impl(ctx):
    ctx.file(
        "BUILD.bazel",
        go_toolchains_build_file_content(
            ctx,
            prefixes = ctx.attr.prefixes,
            geese = ctx.attr.geese,
            goarchs = ctx.attr.goarchs,
            sdk_repos = ctx.attr.sdk_repos,
            sdk_types = ctx.attr.sdk_types,
            sdk_versions = ctx.attr.sdk_versions,
        ),
        executable = False,
    )

go_multiple_toolchains = repository_rule(
    implementation = _go_multiple_toolchains_impl,
    attrs = {
        "prefixes": attr.string_list(mandatory = True),
        "sdk_repos": attr.string_list(mandatory = True),
        "sdk_types": attr.string_list(mandatory = True),
        "sdk_versions": attr.string_list(mandatory = True),
        "geese": attr.string_list(mandatory = True),
        "goarchs": attr.string_list(mandatory = True),
    },
)

def _go_toolchains(name, sdk_repo, sdk_type, sdk_version = None, goos = None, goarch = None):
    go_multiple_toolchains(
        name = name,
        prefixes = [""],
        geese = [goos or ""],
        goarchs = [goarch or ""],
        sdk_repos = [sdk_repo],
        sdk_types = [sdk_type],
        sdk_versions = [sdk_version or ""],
    )

def go_download_sdk(name, register_toolchains = True, **kwargs):
    go_download_sdk_rule(name = name, **kwargs)
    _go_toolchains(
        name = name + "_toolchains",
        sdk_repo = name,
        sdk_type = "remote",
        sdk_version = kwargs.get("version"),
        goos = kwargs.get("goos"),
        goarch = kwargs.get("goarch"),
    )
    if register_toolchains:
        _register_toolchains(name)

def _go_local_sdk_impl(ctx):
    goroot = ctx.attr.path
    platform = _detect_sdk_platform(ctx, goroot)
    version = _detect_sdk_version(ctx, goroot)
    _sdk_build_file(ctx, platform, version, ctx.attr.experiments, ctx.attr.toolchain_experiments)
    _local_sdk(ctx, goroot)

_go_local_sdk = repository_rule(
    implementation = _go_local_sdk_impl,
    attrs = {
        "path": attr.string(),
        "version": attr.string(),
        "experiments": attr.string_list(
            doc = "Go experiments to enable via GOEXPERIMENT",
        ),
        "toolchain_experiments": attr.string_list(
            doc = "Go experiments to enable via GOEXPERIMENT for toolchain tools",
        ),
        "_sdk_build_file": attr.label(
            default = Label("//go/private:BUILD.sdk.bazel"),
        ),
    },
)

def go_local_sdk(name, register_toolchains = True, **kwargs):
    _go_local_sdk(name = name, **kwargs)
    _go_toolchains(
        name = name + "_toolchains",
        sdk_repo = name,
        sdk_type = "remote",
        sdk_version = kwargs.get("version"),
        goos = kwargs.get("goos"),
        goarch = kwargs.get("goarch"),
    )
    if register_toolchains:
        _register_toolchains(name)

def _go_wrap_sdk_impl(ctx):
    if not ctx.attr.root_file and not ctx.attr.root_files:
        fail("either root_file or root_files must be provided")
    if ctx.attr.root_file and ctx.attr.root_files:
        fail("root_file and root_files cannot be both provided")
    if ctx.attr.root_file:
        root_file = ctx.attr.root_file
    else:
        goos, goarch = detect_host_platform(ctx)
        platform = goos + "_" + goarch
        if platform not in ctx.attr.root_files:
            fail("unsupported platform {}".format(platform))
        root_file = Label(ctx.attr.root_files[platform])
    goroot = str(ctx.path(root_file).dirname)
    platform = _detect_sdk_platform(ctx, goroot)
    version = _detect_sdk_version(ctx, goroot)
    _sdk_build_file(ctx, platform, version, ctx.attr.experiments, ctx.attr.toolchain_experiments)
    _local_sdk(ctx, goroot)

# string_keyed_label_dict was added in 8.0.0
_maybe_string_keyed_label_dict = getattr(
    attr,
    "string_keyed_label_dict",
    attr.string_dict,
)
go_wrap_sdk_rule = repository_rule(
    implementation = _go_wrap_sdk_impl,
    attrs = {
        "root_file": attr.label(
            mandatory = False,
            doc = "A file in the SDK root direcotry. Used to determine GOROOT.",
        ),
        "root_files": _maybe_string_keyed_label_dict(
            mandatory = False,
            doc = "A set of mappings from the host platform to a file in the SDK's root directory",
        ),
        "version": attr.string(),
        "experiments": attr.string_list(
            doc = "Go experiments to enable via GOEXPERIMENT",
        ),
        "toolchain_experiments": attr.string_list(
            doc = "Go experiments to enable via GOEXPERIMENT for toolchain tools",
        ),
        "_sdk_build_file": attr.label(
            default = Label("//go/private:BUILD.sdk.bazel"),
        ),
    },
)

def go_wrap_sdk(name, register_toolchains = True, **kwargs):
    goos = kwargs.pop("goos", None)
    goarch = kwargs.pop("goarch", None)
    go_wrap_sdk_rule(name = name, **kwargs)
    _go_toolchains(
        name = name + "_toolchains",
        sdk_repo = name,
        sdk_type = "remote",
        sdk_version = kwargs.get("version"),
        goos = goos,
        goarch = goarch,
    )
    if register_toolchains:
        _register_toolchains(name)

def _register_toolchains(repo):
    native.register_toolchains("@{}_toolchains//:all".format(repo))

def _remote_sdk(ctx, urls, strip_prefix, sha256):
    if len(urls) == 0:
        fail("no urls specified")
    ctx.report_progress("Downloading and extracting Go toolchain")

    auth = use_netrc(read_user_netrc(ctx), urls, {})
    ctx.download_and_extract(
        url = urls,
        stripPrefix = strip_prefix,
        sha256 = sha256,
        auth = auth,
    )

def _local_sdk(ctx, path):
    for entry in ctx.path(path).readdir():
        if ctx.path(entry.basename).exists:
            continue
        ctx.symlink(entry, entry.basename)

def _sdk_build_file(ctx, platform, version, experiments, toolchain_experiments):
    ctx.file("ROOT")
    goos, _, goarch = platform.partition("_")

    ctx.template(
        "BUILD.bazel",
        ctx.path(ctx.attr._sdk_build_file),
        executable = False,
        substitutions = {
            "{goos}": goos,
            "{goarch}": goarch,
            "{exe}": ".exe" if goos == "windows" else "",
            "{version}": version,
            "{experiments}": repr(experiments),
            "{toolchain_experiments}": repr(toolchain_experiments),
            "{exec_compatible_with}": repr([
                GOARCH_CONSTRAINTS[goarch],
                GOOS_CONSTRAINTS[goos],
            ]),
        },
    )

    ctx.file(
        "version.bzl",
        executable = False,
        content = _define_version_constants(version),
    )

def detect_host_platform(ctx):
    goos = ctx.os.name
    if goos == "mac os x":
        goos = "darwin"
    elif goos.startswith("windows"):
        goos = "windows"

    goarch = ctx.os.arch
    if goarch == "aarch64":
        goarch = "arm64"
    elif goarch == "x86_64":
        goarch = "amd64"

    return goos, goarch

def _detect_host_sdk(ctx):
    if "GOROOT" in ctx.os.environ:
        return ctx.os.environ["GOROOT"]
    res = ctx.execute([executable_path(ctx, "go"), "env", "GOROOT"])
    if res.return_code:
        fail("Could not detect host go version")
    root = res.stdout.strip()
    if not root:
        fail("host go version failed to report it's GOROOT")
    return root

def _detect_sdk_platform(ctx, goroot):
    path = ctx.path(goroot + "/pkg/tool")
    if not path.exists:
        fail("Could not detect SDK platform: failed to find " + str(path))
    tool_entries = path.readdir()

    platforms = []
    for f in tool_entries:
        if f.basename.find("_") >= 0:
            platforms.append(f.basename)

    if len(platforms) == 0:
        fail("Could not detect SDK platform: found no platforms in %s" % path)
    if len(platforms) > 1:
        fail("Could not detect SDK platform: found multiple platforms %s in %s" % (platforms, path))
    return platforms[0]

def _detect_sdk_version(ctx, goroot):
    version_file_path = goroot + "/VERSION"
    if ctx.path(version_file_path).exists:
        # VERSION file has version prefixed by go, eg. go1.18.3
        # 1.21: The version is the first line
        version_line = ctx.read(version_file_path).splitlines()[0]
        version = version_line[2:]
        if ctx.attr.version and ctx.attr.version != version:
            fail("SDK is version %s, but version %s was expected" % (version, ctx.attr.version))
        return version

    # The top-level VERSION file does not exist in all Go SDK distributions, e.g. those shipped by Debian or Fedora.
    # Falling back to running "go version"
    go_binary_path = goroot + "/bin/go"
    result = ctx.execute([go_binary_path, "version"])
    if result.return_code != 0:
        fail("Could not detect SDK version: '%s version' exited with exit code %d" % (go_binary_path, result.return_code))

    # go version output is of the form "go version go1.18.3 linux/amd64" or "go
    # version devel go1.19-fd1b5904ae Tue Mar 22 21:38:10 2022 +0000
    # linux/amd64". See the following links for how this output is generated:
    # - https://github.com/golang/go/blob/2bdb5c57f1efcbddab536028d053798e35de6226/src/cmd/go/internal/version/version.go#L75
    # - https://github.com/golang/go/blob/2bdb5c57f1efcbddab536028d053798e35de6226/src/cmd/dist/build.go#L333
    #
    # Read the third word, or the fourth word if the third word is "devel", to
    # find the version number.
    output_parts = result.stdout.split(" ")
    if len(output_parts) > 2 and output_parts[2].startswith("go"):
        version = output_parts[2][len("go"):]
    elif len(output_parts) > 3 and output_parts[2] == "devel" and output_parts[3].startswith("go"):
        version = output_parts[3][len("go"):]
    else:
        fail("Could not parse SDK version from '%s version' output: %s" % (go_binary_path, result.stdout))
    if parse_version(version) == None:
        fail("Could not parse SDK version from '%s version' output: %s" % (go_binary_path, result.stdout))
    if ctx.attr.version and ctx.attr.version != version:
        fail("SDK is version %s, but version %s was expected" % (version, ctx.attr.version))
    return version

def _parse_versions_json(data):
    """Parses version metadata returned by go.dev.

    Args:
        data: the contents of the file downloaded from
            https://go.dev/dl/?mode=json. We assume the file is valid
            JSON, is spaced and indented, and is in a particular format.

    Return:
        A dict mapping version strings (like "1.15.5") to dicts mapping
        platform names (like "linux_amd64") to pairs of filenames
        (like "go1.15.5.linux-amd64.tar.gz") and hex-encoded SHA-256 sums.
    """
    sdks = json.decode(data)
    return {
        sdk["version"][len("go"):]: {
            "%s_%s" % (file["os"], file["arch"]): (
                file["filename"],
                file["sha256"],
            )
            for file in sdk["files"]
            if file["kind"] == "archive"
        }
        for sdk in sdks
    }

def fetch_sdks_by_version(ctx, allow_fail = False):
    result = ctx.download(
        url = [
            "https://go.dev/dl/?mode=json&include=all",
            "https://golang.google.cn/dl/?mode=json&include=all",
        ],
        output = "versions.json",
        allow_fail = allow_fail,
    )
    if not result.success:
        return None
    data = ctx.read("versions.json")

    # If the download is redirected through a proxy such as Artifactory, it may
    # drop the query parameters and return an HTML page instead. In that case,
    # just return an empty map if allow_fail is set. It is unfortunately not
    # possible to attempt parsing as JSON and catch the error.
    if (not data or data[0] != "[") and allow_fail:
        return None

    # module_ctx doesn't have delete, but its files are temporary anyway.
    if hasattr(ctx, "delete"):
        ctx.delete("versions.json")
    return _parse_versions_json(data)

def parse_version(version):
    """Parses a version string like "1.15.5" and returns a tuple of numbers or None"""
    l, r = 0, 0
    parsed = []
    for c in version.elems():
        if c == ".":
            if l == r:
                # empty component
                return None
            parsed.append(int(version[l:r]))
            r += 1
            l = r
            continue

        if c.isdigit():
            r += 1
            continue

        # pre-release suffix
        break

    if l == r:
        # empty component
        return None
    parsed.append(int(version[l:r]))
    if len(parsed) == 2:
        # first minor version, like (1, 15)
        parsed.append(0)
    if len(parsed) != 3:
        # too many or too few components
        return None
    if r < len(version):
        # pre-release suffix
        parsed.append(version[r:])
    return tuple(parsed)

def _version_is_prerelease(v):
    return len(v) > 3

def _version_less(a, b):
    if a[:3] < b[:3]:
        return True
    if a[:3] > b[:3]:
        return False
    if len(a) > len(b):
        return True
    if len(a) < len(b) or len(a) == 3:
        return False
    return a[3:] < b[3:]

def _version_string(v):
    suffix = v[3] if _version_is_prerelease(v) else ""
    return ".".join([str(n) for n in v]) + suffix

def _have_same_length(*lists):
    if not lists:
        fail("expected at least one list")
    return len({len(l): None for l in lists}) == 1

def go_register_toolchains(version = None, nogo = None, go_version = None, experiments = None, toolchain_experiments = None):
    """See /go/toolchains.rst#go-register-toolchains for full documentation."""
    if not version:
        version = go_version  # old name

    sdk_kinds = ("go_download_sdk_rule", "go_host_sdk_rule", "_go_local_sdk", "go_wrap_sdk_rule")
    existing_rules = native.existing_rules()
    sdk_rules = [r for r in existing_rules.values() if r["kind"] in sdk_kinds]
    if len(sdk_rules) == 0 and "go_sdk" in existing_rules:
        # may be local_repository in bazel_tests.
        sdk_rules.append(existing_rules["go_sdk"])

    if version and len(sdk_rules) > 0:
        fail("go_register_toolchains: version set after go sdk rule declared ({})".format(", ".join([r["name"] for r in sdk_rules])))
    if len(sdk_rules) == 0:
        if not version:
            fail('go_register_toolchains: version must be a string like "1.15.5" or "host"')
        elif version == "host":
            go_host_sdk(name = "go_sdk", experiments = experiments, toolchain_experiments = toolchain_experiments)
        else:
            pv = parse_version(version)
            if not pv:
                fail('go_register_toolchains: version must be a string like "1.15.5" or "host"')
            if _version_less(pv, MIN_SUPPORTED_VERSION):
                print("DEPRECATED: Go versions before {} are not supported and may not work".format(_version_string(MIN_SUPPORTED_VERSION)))
            go_download_sdk(
                name = "go_sdk",
                version = version,
                experiments = experiments,
                toolchain_experiments = toolchain_experiments,
            )

    if nogo:
        # Override default definition in go_rules_dependencies().
        go_register_nogo(
            name = "io_bazel_rules_nogo",
            nogo = nogo,
        )
