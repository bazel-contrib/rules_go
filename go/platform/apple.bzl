# Copyright 2018 The Bazel Authors. All rights reserved.
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

_PLATFORMS = {
    "armv7-apple-ios": (apple_common.platform.ios_device, apple_common.platform_type.ios),
    "armv7-apple-tvos": (apple_common.platform.tvos_device, apple_common.platform_type.tvos),
    "armv7k-apple-watchos": (apple_common.platform.watchos_device, apple_common.platform_type.watchos),
    "arm64-apple-ios": (apple_common.platform.ios_device, apple_common.platform_type.ios),
    "arm64-apple-tvos": (apple_common.platform.tvos_device, apple_common.platform_type.tvos),
    "i386-apple-ios": (apple_common.platform.ios_simulator, apple_common.platform_type.ios),
    "i386-apple-tvos": (apple_common.platform.tvos_simulator, apple_common.platform_type.tvos),
    "i386-apple-watchos": (apple_common.platform.watchos_simulator, apple_common.platform_type.watchos),
    "x86_64-apple-ios": (apple_common.platform.ios_simulator, apple_common.platform_type.ios),
    "x86_64-apple-tvos": (apple_common.platform.ios_simulator, apple_common.platform_type.tvos),
    "x86_64-apple-watchos": (apple_common.platform.watchos_simulator, apple_common.platform_type.watchos),
}

def _apple_version_min(ctx, platform, platform_type):
    xcode_config = ctx.attr._xcode_config[apple_common.XcodeVersionConfig]
    min_os = str(xcode_config.minimum_os_for_platform_type(platform_type))
    return "-m{}-version-min={}".format(platform.name_in_plist.lower(), min_os)

def _apple_env(ctx, platform):
    xcode_config = ctx.attr._xcode_config[apple_common.XcodeVersionConfig]
    return apple_common.target_apple_env(xcode_config, platform)

def apple_ensure_options(ctx, env, compiler_option_lists, linker_option_lists, target_gnu_system_name):
    """Returns environment and flags for Apple targets."""
    platform, platform_type = _PLATFORMS.get(target_gnu_system_name, (None, None))
    if not platform:
        return
    env.update(_apple_env(ctx, platform))
    min_version = _apple_version_min(ctx, platform, platform_type)
    for compiler_options in compiler_option_lists:
        compiler_options.append(min_version)
        # Allow unused parameters since they are present in the stdlib src.
        compiler_options.append("-Wno-unused-parameter")
    for linker_options in linker_option_lists:
        linker_options.append(min_version)
