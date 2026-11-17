# Copyright 2026 The Bazel Authors. All rights reserved.
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
"""
Declares the toolchain targets of a Go SDK.

This is kept apart from go_toolchain.bzl so that loading the registered
toolchains, which Bazel does for any toolchain resolution, does not load the
Go rules and with them @io_bazel_rules_nogo.
"""

load("@bazel_skylib//lib:selects.bzl", "selects")
load("//go/private:common.bzl", "GO_TOOLCHAIN")
load("//go/private:platforms.bzl", "PLATFORMS")

def declare_bazel_toolchains(
        *,
        go_toolchain_repo,
        exec_goarch,
        exec_goos,
        major,
        minor,
        patch,
        prerelease,
        sdk_name,
        sdk_type,
        prefix = ""):
    """Declares toolchain targets for each platform."""

    sdk_version_label = Label("//go/toolchain:sdk_version")

    native.config_setting(
        name = prefix + "match_all_versions",
        flag_values = {
            sdk_version_label: "",
        },
        visibility = ["//visibility:private"],
    )

    native.config_setting(
        name = prefix + "match_major_version",
        flag_values = {
            sdk_version_label: major,
        },
        visibility = ["//visibility:private"],
    )

    native.config_setting(
        name = prefix + "match_major_minor_version",
        flag_values = {
            sdk_version_label: major + "." + minor,
        },
        visibility = ["//visibility:private"],
    )

    native.config_setting(
        name = prefix + "match_patch_version",
        flag_values = {
            sdk_version_label: major + "." + minor + "." + patch,
        },
        visibility = ["//visibility:private"],
    )

    # If prerelease version is "", this will be the same as ":match_patch_version", but that's fine since we use match_any in config_setting_group.
    native.config_setting(
        name = prefix + "match_prerelease_version",
        flag_values = {
            sdk_version_label: major + "." + minor + "." + patch + prerelease,
        },
        visibility = ["//visibility:private"],
    )

    native.config_setting(
        name = prefix + "match_minor_release_candidate",
        flag_values = {
            sdk_version_label: major + "." + minor + prerelease,
        },
        visibility = ["//visibility:private"],
    )

    native.config_setting(
        name = prefix + "match_sdk_type",
        flag_values = {
            sdk_version_label: sdk_type,
        },
        visibility = ["//visibility:private"],
    )

    selects.config_setting_group(
        name = prefix + "sdk_version_setting",
        match_any = [
            ":" + prefix + "match_all_versions",
            ":" + prefix + "match_major_version",
            ":" + prefix + "match_major_minor_version",
            ":" + prefix + "match_patch_version",
            ":" + prefix + "match_prerelease_version",
            ":" + prefix + "match_minor_release_candidate",
            ":" + prefix + "match_sdk_type",
        ],
        visibility = ["//visibility:private"],
    )

    # use Label constructor to resolve the label relative to the package this bzl file
    # is located, instead of the context of the caller of declare_bazel_toolchains.
    # See https://bazel.build/rules/lib/builtins/Label#Label for details.
    sdk_name_label = Label("//go/toolchain:sdk_name")

    native.config_setting(
        name = prefix + "match_sdk_name",
        flag_values = {
            sdk_name_label: sdk_name,
        },
        visibility = ["//visibility:private"],
    )

    native.config_setting(
        name = prefix + "match_all_sdks",
        flag_values = {
            sdk_name_label: "",
        },
        visibility = ["//visibility:private"],
    )

    selects.config_setting_group(
        name = prefix + "sdk_name_setting",
        match_any = [
            ":" + prefix + "match_all_sdks",
            ":" + prefix + "match_sdk_name",
        ],
        visibility = ["//visibility:private"],
    )

    for p in PLATFORMS:
        if p.cgo:
            # Don't declare separate toolchains for cgo_on / cgo_off.
            continue

        cgo_constraints = (
            "@io_bazel_rules_go//go/toolchain:cgo_off",
            "@io_bazel_rules_go//go/toolchain:cgo_on",
        )
        constraints = [c for c in p.constraints if c not in cgo_constraints]

        native.toolchain(
            # keep in sync with generate_toolchain_names
            name = prefix + "go_" + p.name,
            toolchain_type = GO_TOOLCHAIN,
            exec_compatible_with = [
                "@io_bazel_rules_go//go/toolchain:" + exec_goos,
                "@io_bazel_rules_go//go/toolchain:" + exec_goarch,
            ],
            target_compatible_with = constraints,
            target_settings = [
                ":" + prefix + "sdk_name_setting",
                ":" + prefix + "sdk_version_setting",
            ],
            toolchain = go_toolchain_repo + "//:go_" + p.name + "-impl",
        )
