# Copyright 2020 The Bazel Authors. All rights reserved.
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
    "//go/private:mode.bzl",
    "LINKMODES",
    "LINKMODE_NORMAL",
)
load(
    "//go/private:platforms.bzl",
    "CGO_GOOS_GOARCH",
    "GOOS_GOARCH",
)
load(
    "//go/private:providers.bzl",
    "GoArchive",
    "GoInfo",
)

# A list of rules_go settings that are possibly set by go_transition.
# Keep their package name in sync with the implementation of
# _original_setting_key.
TRANSITIONED_GO_SETTING_KEYS = [
    "//go/config:static",
    "//go/config:msan",
    "//go/config:race",
    "//go/config:pure",
    "//go/config:linkmode",
    "//go/config:tags",
    "//go/config:pgoprofile",
]

def _deduped_and_sorted(strs):
    return sorted({s: None for s in strs}.keys())

def _original_setting_key(key):
    if not "//go/config:" in key:
        fail("_original_setting_key currently assumes that all Go settings live under //go/config, got: " + key)
    name = key.split(":")[1]
    return "//go/private/rules:original_" + name

_SETTING_KEY_TO_ORIGINAL_SETTING_KEY = {
    setting: _original_setting_key(setting)
    for setting in TRANSITIONED_GO_SETTING_KEYS
}

def _go_transition_impl(settings, attr):
    # NOTE: Keep the list of rules_go settings set by this transition in sync
    # with POTENTIALLY_TRANSITIONED_SETTINGS.
    #
    # NOTE(bazelbuild/bazel#11409): Calling fail here for invalid combinations
    # of flags reports an error but does not stop the build.
    # In any case, validate_mode should mainly be responsible for reporting
    # invalid modes, since it also takes --features flags into account.

    original_settings = settings
    settings = dict(settings)

    _set_ternary(settings, attr, "static")
    race = _set_ternary(settings, attr, "race")
    msan = _set_ternary(settings, attr, "msan")
    pure = _set_ternary(settings, attr, "pure")
    if race == "on":
        if pure == "on":
            fail('race = "on" cannot be set when pure = "on" is set. race requires cgo.')
        pure = "off"
        settings["//go/config:pure"] = False
    if msan == "on":
        if pure == "on":
            fail('msan = "on" cannot be set when msan = "on" is set. msan requires cgo.')
        pure = "off"
        settings["//go/config:pure"] = False
    if pure == "on":
        settings["//go/config:race"] = False
        settings["//go/config:msan"] = False
    cgo = pure == "off"

    goos = getattr(attr, "goos", "auto")
    goarch = getattr(attr, "goarch", "auto")
    _check_ternary("pure", pure)
    if goos != "auto" or goarch != "auto":
        if goos == "auto":
            fail("goos must be set if goarch is set")
        if goarch == "auto":
            fail("goarch must be set if goos is set")
        if (goos, goarch) not in GOOS_GOARCH:
            fail("invalid goos, goarch pair: {}, {}".format(goos, goarch))
        if cgo and (goos, goarch) not in CGO_GOOS_GOARCH:
            fail('pure is "off" but cgo is not supported on {} {}'.format(goos, goarch))
        platform = "@io_bazel_rules_go//go/toolchain:{}_{}{}".format(goos, goarch, "_cgo" if cgo else "")
        settings["//command_line_option:platforms"] = platform

    tags = getattr(attr, "gotags", [])
    if tags:
        settings["//go/config:tags"] = _deduped_and_sorted(tags)

    linkmode = getattr(attr, "linkmode", "auto")
    if linkmode != "auto":
        if linkmode not in LINKMODES:
            fail("linkmode: invalid mode {}; want one of {}".format(linkmode, ", ".join(LINKMODES)))
        settings["//go/config:linkmode"] = linkmode

    pgoprofile = getattr(attr, "pgoprofile", "auto")
    if pgoprofile != "auto":
        settings["//go/config:pgoprofile"] = pgoprofile

    for key, original_key in _SETTING_KEY_TO_ORIGINAL_SETTING_KEY.items():
        old_value = original_settings[key]
        value = settings[key]

        # If the outgoing configuration would differ from the incoming one in a
        # value, record the old value in the special original_* key so that the
        # real setting can be reset to this value before the new configuration
        # would cross a non-deps dependency edge.
        if value != old_value:
            if original_settings.get(original_key):
                fail("go_transition can't be nested")

            # Encoding as JSON makes it possible to embed settings of arbitrary
            # types (currently bool, string and string_list) into a single type
            # of setting (string) with the information preserved whether the
            # original setting wasn't set explicitly (empty string) or was set
            # explicitly to its default  (always a non-empty string with JSON
            # encoding, e.g. "\"\"" or "[]").
            if type(old_value) == "Label":
                # Label is not JSON serializable, so we need to convert it to a string.
                old_value = str(old_value)
            settings[original_key] = json.encode(old_value)
        else:
            # Preserve the value to keep the transition idempotent. While it
            # should never be nested, Bazel applies it twice to check for
            # idempotency and in Bazel 8.3.1 cquery and aquery don't handle
            # targets with non-idempotent rule transitions correctly.
            # https://github.com/bazelbuild/bazel/pull/26738
            settings[original_key] = original_settings.get(original_key, "")

    return settings

go_transition = transition(
    implementation = _go_transition_impl,
    inputs = [
        "//command_line_option:platforms",
    ] + TRANSITIONED_GO_SETTING_KEYS + _SETTING_KEY_TO_ORIGINAL_SETTING_KEY.values(),
    outputs = [
        "//command_line_option:platforms",
    ] + TRANSITIONED_GO_SETTING_KEYS + _SETTING_KEY_TO_ORIGINAL_SETTING_KEY.values(),
)

def _request_nogo_transition(settings, _attr):
    """Indicates that we want the project configured nogo instead of a noop.

    This does not guarantee that the project configured nogo will be used (if
    bootstrap is true we are currently building nogo so that is a cyclic
    dependency).

    The config setting nogo_active requires bootstrap to be false and
    request_nogo to be true to provide the project configured nogo.
    """
    settings = dict(settings)
    settings["//go/private:request_nogo"] = True
    return settings

request_nogo_transition = transition(
    implementation = _request_nogo_transition,
    inputs = [],
    outputs = ["//go/private:request_nogo"],
)

def _non_request_nogo_transition(_settings, _attr):
    # This transition is used to make sure we only end up with 1 copy of coverdata,
    # even if a test links against it and is run in coverage mode.
    return {"//go/private:request_nogo": False}

non_request_nogo_transition = transition(
    implementation = _non_request_nogo_transition,
    inputs = [],
    outputs = ["//go/private:request_nogo"],
)

_common_reset_transition_dict = dict({
    "//go/private:request_nogo": False,
    "//go/config:static": False,
    "//go/config:msan": False,
    "//go/config:race": False,
    "//go/config:pure": False,
    "//go/config:debug": False,
    "//go/config:linkmode": LINKMODE_NORMAL,
    "//go/config:tags": [],
    "//go/config:pgoprofile": Label("//go/config:empty"),
}, **{setting: "" for setting in _SETTING_KEY_TO_ORIGINAL_SETTING_KEY.values()})

_reset_transition_dict = dict(_common_reset_transition_dict, **{
    "//go/private:bootstrap_nogo": True,
})

_reset_transition_keys = sorted(_reset_transition_dict.keys())

_stdlib_keep_keys = sorted([
    "//go/config:msan",
    "//go/config:race",
    "//go/config:pure",
    "//go/config:linkmode",
    "//go/config:tags",
    "//go/config:pgoprofile",
])

def _go_tool_transition_impl(settings, _attr):
    """Sets most Go settings to default values (use for external Go tools).

    go_tool_transition sets all of the //go/config settings to their default
    values and disables nogo. This is used for Go tool binaries like nogo
    itself. Tool binaries shouldn't depend on the link mode or tags of the
    target configuration and neither the tools nor the code they potentially
    generate should be subject to nogo's static analysis. This transition
    doesn't change the platform (goos, goarch), but tool binaries should also
    have `cfg = "exec"` so tool binaries should be built for the execution
    platform.
    """
    return dict(settings, **_reset_transition_dict)

go_tool_transition = transition(
    implementation = _go_tool_transition_impl,
    inputs = _reset_transition_keys,
    outputs = _reset_transition_keys,
)

def _non_go_tool_transition_impl(settings, _attr):
    """Sets all Go settings to default values (use for external non-Go tools).

    non_go_tool_transition sets all of the //go/config settings as well as the
    nogo settings to their default values. This is used for all tools that are
    not themselves targets created from rules_go rules and thus do not read
    these settings. Resetting all of them to defaults prevents unnecessary
    configuration changes for these targets that could cause rebuilds.

    Examples: This transition is applied to attributes referencing proto_library
    targets or protoc directly.
    """
    settings = dict(settings, **_reset_transition_dict)
    settings["//go/private:bootstrap_nogo"] = False
    return settings

non_go_tool_transition = transition(
    implementation = _non_go_tool_transition_impl,
    inputs = _reset_transition_keys,
    outputs = _reset_transition_keys,
)

def _go_stdlib_transition_impl(settings, _attr):
    """Sets all Go settings to their default values, except for those affecting the Go SDK.

    This transition is similar to _non_go_tool_transition except that it keeps the
    parts of the configuration that determine how to build the standard library.
    It's used to consolidate the configurations used to build the standard library to limit
    the number built.
    """
    settings = dict(settings)
    for label, value in _reset_transition_dict.items():
        if label not in _stdlib_keep_keys:
            settings[label] = value
    settings["//go/config:tags"] = [t for t in settings["//go/config:tags"] if t in _TAG_AFFECTS_STDLIB]
    settings["//go/private:bootstrap_nogo"] = False
    settings["//command_line_option:collect_code_coverage"] = False
    return settings

go_stdlib_transition = transition(
    implementation = _go_stdlib_transition_impl,
    inputs = _reset_transition_keys,
    outputs = _reset_transition_keys + ["//command_line_option:collect_code_coverage"],
)

def _go_reset_target_impl(ctx):
    t = ctx.attr.dep[0]  # [0] seems to be necessary with the transition
    providers = [t[p] for p in [GoInfo, GoArchive] if p in t]

    # We can't pass DefaultInfo through as-is, since Bazel forbids executable
    # if it's a file declared in a different target. To emulate that, symlink
    # to the original executable, if there is one.
    default_info = t[DefaultInfo]

    new_executable = None
    original_executable = default_info.files_to_run.executable
    default_runfiles = default_info.default_runfiles
    if original_executable:
        # In order for the symlink to have the same basename as the original
        # executable (important in the case of proto plugins), put it in a
        # subdirectory named after the label to prevent collisions.
        new_executable = ctx.actions.declare_file(paths.join(ctx.label.name, original_executable.basename))
        ctx.actions.symlink(
            output = new_executable,
            target_file = original_executable,
            is_executable = True,
        )
        default_runfiles = default_runfiles.merge(ctx.runfiles([new_executable]))

    providers.append(
        DefaultInfo(
            files = default_info.files,
            data_runfiles = default_info.data_runfiles,
            default_runfiles = default_runfiles,
            executable = new_executable,
        ),
    )
    return providers

go_reset_target = rule(
    implementation = _go_reset_target_impl,
    attrs = {
        "dep": attr.label(
            mandatory = True,
            cfg = go_tool_transition,
            doc = """The target to forward providers from and apply go_tool_transition to.""",
        ),
        "_allowlist_function_transition": attr.label(
            default = "@bazel_tools//tools/allowlists/function_transition_allowlist",
        ),
    },
    doc = """Forwards providers from a target and default Go binary settings.

go_reset_target depends on a single target and builds it to be a Go tool binary. It
forwards Go providers and DefaultInfo.

go_reset_target does two things using transitions:
   1. builds the tool with 'cfg = "exec"' so they work on the execution platform.
   2. Sets most Go settings to default value and disables nogo.

This is used for Go tool binaries that shouldn't depend on the link mode or tags of the
target configuration and neither the tools nor the code they potentially
generate should be subject to Nogo's static analysis. This is helpful, for example, so
a tool isn't built as a shared library with race instrumentation. This acts as an
intermediate rule that allows users to apply these transitions.
""",
)

non_go_reset_target = rule(
    implementation = _go_reset_target_impl,
    attrs = {
        "dep": attr.label(
            mandatory = True,
            cfg = non_go_tool_transition,
            allow_files = True,
        ),
        "_allowlist_function_transition": attr.label(
            default = "@bazel_tools//tools/allowlists/function_transition_allowlist",
        ),
    },
    doc = """Forwards providers from a target and applies non_go_tool_transition.

non_go_reset_target depends on a single target, built using
non_go_tool_transition. It forwards Go providers and DefaultInfo.

This is used to work around a problem with building tools: Non-Go tools should
be built with 'cfg = "exec"' so they work on the execution platform, but they
also shouldn't be affected by Go-specific config changes applied by
go_transition.
""",
)

def _non_go_transition_impl(settings, _attr):
    """Sets all Go settings to the values they had before the last go_transition.

    non_go_transition sets all of the //go/config settings to the value they had
    before the last go_transition. This should be used on all attributes of
    go_library/go_binary/go_test that are built in the target configuration and
    do not constitute advertise any Go providers.

    Examples: This transition is applied to the 'data' attribute of go_binary so
    that other Go binaries used at runtime aren't affected by a non-standard
    link mode set on the go_binary target, but still use the same top-level
    settings such as e.g. race instrumentation.
    """
    new_settings = {}
    for key, original_key in _SETTING_KEY_TO_ORIGINAL_SETTING_KEY.items():
        original_value = settings[original_key]
        if original_value:
            # Reset to the original value of the setting before go_transition.
            new_settings[key] = json.decode(original_value)
        else:
            new_settings[key] = settings[key]

        # Reset the value of the helper setting to its default for two reasons:
        # 1. Performance: This ensures that the Go settings of non-Go
        #    dependencies have the same values as before the go_transition,
        #    which can prevent unnecessary rebuilds caused by configuration
        #    changes.
        # 2. Correctness in edge cases: If there is a path in the build graph
        #    from a go_binary's non-Go dependency to a go_library that does not
        #    pass through another go_binary (e.g., through a custom rule
        #    replacement for go_binary), this transition could be applied again
        #    and cause incorrect Go setting values.
        new_settings[original_key] = ""

    return new_settings

non_go_transition = transition(
    implementation = _non_go_transition_impl,
    inputs = TRANSITIONED_GO_SETTING_KEYS + _SETTING_KEY_TO_ORIGINAL_SETTING_KEY.values(),
    outputs = TRANSITIONED_GO_SETTING_KEYS + _SETTING_KEY_TO_ORIGINAL_SETTING_KEY.values(),
)

def _check_ternary(name, value):
    if value not in ("on", "off", "auto"):
        fail('{}: must be "on", "off", or "auto"'.format(name))

def _set_ternary(settings, attr, name):
    value = getattr(attr, name, "auto")
    _check_ternary(name, value)
    if value != "auto":
        label = "//go/config:{}".format(name)
        settings[label] = value == "on"
    return value

_SDK_VERSION_BUILD_SETTING = "//go/toolchain:sdk_version"
TRANSITIONED_GO_CROSS_SETTING_KEYS = [
    _SDK_VERSION_BUILD_SETTING,
    "//command_line_option:compilation_mode",
    "//command_line_option:platforms",
]

def _go_cross_transition_impl(settings, attr):
    settings = dict(settings)
    if attr.sdk_version != None:
        settings[_SDK_VERSION_BUILD_SETTING] = attr.sdk_version

    if attr.compilation_mode != "":
        settings["//command_line_option:compilation_mode"] = attr.compilation_mode

    if attr.platform != None:
        settings["//command_line_option:platforms"] = str(attr.platform)

    return settings

go_cross_transition = transition(
    implementation = _go_cross_transition_impl,
    inputs = TRANSITIONED_GO_CROSS_SETTING_KEYS,
    outputs = TRANSITIONED_GO_CROSS_SETTING_KEYS,
)

# A list of Go build tags that potentially affect the build of the standard
# library.
#
# This should be updated to contain the union of all tags relevant for all
# versions of Go that are still relevant.
#
# Currently supported versions: 1.18..1.23
#
# To regenerate, run and paste the output of
#     bazel run //go/tools/internal/stdlib_tags:stdlib_tags -- path/to/go_sdk_1/src ...
_TAG_AFFECTS_STDLIB = {
    "alpha": None,
    "appengine": None,
    "asan": None,
    "boringcrypto": None,  # Added in Go 1.19
    "checknewoldreassignment": None,  # Added in Go 1.22
    "cmd_go_bootstrap": None,
    "compiler_bootstrap": None,
    "debuglog": None,
    "debugtrace": None,  # Added in Go 1.22
    "faketime": None,
    "gc": None,
    "gccgo": None,
    "gen": None,  # Removed in Go 1.20
    "generate": None,
    "gofuzz": None,  # Removed in Go 1.23
    "icu": None,  # Added in Go 1.23
    "ignore": None,
    "internal": None,  # Added in Go 1.21
    "internal_pie": None,  # Added in Go 1.21, removed in Go 1.22
    "libfuzzer": None,
    "m68k": None,
    "math_big_pure_go": None,
    "msan": None,
    "netcgo": None,
    "netgo": None,
    "nethttpomithttp2": None,
    "nios2": None,
    "noopt": None,  # Added in Go 1.20
    "osusergo": None,
    "purego": None,
    "race": None,
    "sh": None,
    "shbe": None,
    "static": None,  # Added in Go 1.21
    "tablegen": None,  # Removed in Go 1.19
    "testgo": None,  # Removed in Go 1.19
    "timetzdata": None,
    "tools": None,  # Added in Go 1.21
}
