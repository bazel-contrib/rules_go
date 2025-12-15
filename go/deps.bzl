# Copyright 2019 The Bazel Authors. All rights reserved.
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

# deps.bzl loads definitions for use in WORKSPACE files. It's important
# to keep this file and the .bzl files it loads separate from the files
# loaded by def.bzl. def.bzl and its dependencies may depend on repositories
# declared here, but at the time this file is loaded, we can't assume
# anything has been declared.

load(
    "//go/private:nogo.bzl",
    "go_register_nogo_wrapper",
)
load(
    "//go/private:repositories.bzl",
    _go_rules_dependencies = "go_rules_dependencies",
)
load(
    "//go/private:sdk.bzl",
    _go_download_sdk = "go_download_sdk",
    _go_host_sdk = "go_host_sdk",
    _go_local_sdk = "go_local_sdk",
    _go_register_toolchains = "go_register_toolchains",
    _go_wrap_sdk = "go_wrap_sdk",
)

go_rules_dependencies = _go_rules_dependencies
go_register_toolchains = _go_register_toolchains
go_download_sdk = _go_download_sdk
go_host_sdk = _go_host_sdk
go_local_sdk = _go_local_sdk
go_wrap_sdk = _go_wrap_sdk
go_register_nogo = go_register_nogo_wrapper
