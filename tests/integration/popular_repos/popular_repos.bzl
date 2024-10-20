# Copyright 2017 The Bazel Authors. All rights reserved.
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

##############################
# Generated file, do not edit!
##############################

load("@bazel_gazelle//:def.bzl", "go_repository")

def _maybe(repo_rule, name, **kwargs):
    if name not in native.existing_rules():
        repo_rule(name = name, **kwargs)

def popular_repos():
    _maybe(
        go_repository,
        name = "org_golang_x_crypto",
        importpath = "golang.org/x/crypto",
        commit = "0d375be9b61cb69eb94173d0375a05e90875bbf6",
    )
    _maybe(
        go_repository,
        name = "org_golang_x_net",
        importpath = "golang.org/x/net",
        commit = "e18ecbb051101a46fc263334b127c89bc7bff7ea",
    )
    _maybe(
        go_repository,
        name = "org_golang_x_sys",
        importpath = "golang.org/x/sys",
        commit = "390168757d9c647283340d526204e3409d5903f3",
    )
    _maybe(
        go_repository,
        name = "org_golang_x_text",
        importpath = "golang.org/x/text",
        commit = "e3aa4adf54f644ca0cb35f1f1fb19b239c40ef04",
    )
    _maybe(
        go_repository,
        name = "org_golang_x_tools",
        importpath = "golang.org/x/tools",
        commit = "fe37c9e135b934191089b245ac29325091462508",
    )
    _maybe(
        go_repository,
        name = "com_github_golang_glog",
        importpath = "github.com/golang/glog",
        commit = "23def4e6c14b4da8ac2ed8007337bc5eb5007998",
    )
    _maybe(
        go_repository,
        name = "org_golang_x_sync",
        importpath = "golang.org/x/sync",
        commit = "036812b2e83c0ddf193dd5a34e034151da389d09",
    )
    _maybe(
        go_repository,
        name = "org_golang_x_mod",
        importpath = "golang.org/x/mod",
        commit = "86c51ed26bb44749b7d60a57bab0e7524656fe8a",
    )
