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

# Once nested repositories work, this file should cease to exist.

load("@bazel_tools//tools/build_defs/repo:http.bzl", "http_archive")
load("//go/private:common.bzl", "MINIMUM_BAZEL_VERSION")
load("//go/private:nogo.bzl", "DEFAULT_NOGO", "go_register_nogo")
load("//go/private:polyfill_bazel_features.bzl", "polyfill_bazel_features")
load("//go/private/skylib/lib:versions.bzl", "versions")
load("//proto:gogo.bzl", "gogo_special_proto")

def go_rules_dependencies(force = False):
    """Declares workspaces the Go rules depend on. Workspaces that use
    rules_go should call this.

    See https://github.com/bazelbuild/rules_go/blob/master/go/dependencies.rst#overriding-dependencies
    for information on each dependency.

    Instructions for updating this file are in
    https://github.com/bazelbuild/rules_go/wiki/Updating-dependencies.

    PRs updating dependencies are NOT ACCEPTED. See
    https://github.com/bazelbuild/rules_go/blob/master/go/dependencies.rst#overriding-dependencies
    for information on choosing different versions of these repositories
    in your own project.
    """
    if getattr(native, "bazel_version", None):
        versions.check(MINIMUM_BAZEL_VERSION, bazel_version = native.bazel_version)

    if force:
        wrapper = _always
    else:
        wrapper = _maybe

    # Needed by rules_go implementation and tests.
    # We can't call bazel_skylib_workspace from here. At the moment, it's only
    # used to register unittest toolchains, which rules_go does not need.
    # releaser:upgrade-dep bazelbuild bazel-skylib
    wrapper(
        http_archive,
        name = "bazel_skylib",
        # 1.6.1, latest as of 2024-05-20
        urls = [
            "https://mirror.bazel.build/github.com/bazelbuild/bazel-skylib/releases/download/1.6.1/bazel-skylib-1.6.1.tar.gz",
            "https://github.com/bazelbuild/bazel-skylib/releases/download/1.6.1/bazel-skylib-1.6.1.tar.gz",
        ],
        sha256 = "9f38886a40548c6e96c106b752f242130ee11aaa068a56ba7e56f4511f33e4f2",
        strip_prefix = "",
    )

    # Needed for nogo vet checks and go/packages.
    # releaser:upgrade-dep golang tools
    wrapper(
        http_archive,
        name = "org_golang_x_tools",
        # v0.30.0, latest as of 2025-02-13
        urls = [
            "https://mirror.bazel.build/github.com/golang/tools/archive/refs/tags/v0.30.0.zip",
            "https://github.com/golang/tools/archive/refs/tags/v0.30.0.zip",
        ],
        sha256 = "0736b1a0aa28f48074891a0f93cef5396575dbd73b9b5cdc4de54b2a3bfa4b4b",
        strip_prefix = "tools-0.30.0",
        patches = [
            # deletegopls removes the gopls subdirectory. It contains a nested
            # module with additional dependencies. It's not needed by rules_go.
            # releaser:patch-cmd rm -rf gopls
            Label("//third_party:org_golang_x_tools-deletegopls.patch"),
            # releaser:patch-cmd gazelle -repo_root . -go_prefix golang.org/x/tools -go_naming_convention import_alias
            Label("//third_party:org_golang_x_tools-gazelle.patch"),
        ],
        patch_args = ["-p1"],
    )

    # Needed for go/tools/fetch_repo
    # releaser:upgrade-dep golang tools go vcs
    wrapper(
        http_archive,
        name = "org_golang_x_tools_go_vcs",
        # v0.12.0, latest as of 2023-08-12
        urls = [
            "https://mirror.bazel.build/github.com/golang/tools/archive/refs/tags/go/vcs/v0.1.0-deprecated.zip",
            "https://github.com/golang/tools/archive/refs/tags/go/vcs/v0.1.0-deprecated.zip",
        ],
        sha256 = "1b389268d126467105305ae4482df0189cc80a13aaab28d0946192b4ad0737a8",
        strip_prefix = "tools-go-vcs-v0.1.0-deprecated/go/vcs",
        patches = [
            # releaser:patch-cmd gazelle -repo_root . -go_prefix golang.org/x/tools/go/vcs -go_naming_convention import_alias
            Label("//third_party:org_golang_x_tools_go_vcs-gazelle.patch"),
        ],
        patch_args = ["-p1"],
    )

    # Needed for nogo to generate unified diff
    # releaser:upgrade-dep pmezard go-difflib
    wrapper(
        http_archive,
        name = "com_github_pmezard_go_difflib",
        # v1.0.0, latest as of 2024-12-19
        urls = [
            "https://mirror.bazel.build/github.com/pmezard/go-difflib/archive/refs/tags/v1.0.0.tar.gz",
            "https://github.com/pmezard/go-difflib/archive/refs/tags/v1.0.0.tar.gz",
        ],
        sha256 = "28f3dc1b5c0efd61203ab07233f774740d3bf08da4d8153fb5310db6cea0ebda",
        strip_prefix = "go-difflib-1.0.0",
        patches = [
            # releaser:patch-cmd gazelle -repo_root . -go_prefix github.com/pmezard/go-difflib -go_naming_convention import_alias
            Label("//third_party:com_github_pmezard_go_difflib-gazelle.patch"),
        ],
        patch_args = ["-p1"],
    )

    # releaser:upgrade-dep golang sys
    wrapper(
        http_archive,
        name = "org_golang_x_sys",
        # v0.30.0, from 2025-01-31
        urls = [
            "https://mirror.bazel.build/github.com/golang/sys/archive/refs/tags/v0.30.0.zip",
            "https://github.com/golang/sys/archive/refs/tags/v0.30.0.zip",
        ],
        sha256 = "4acf3387a5ab61b6e2af0463491d0c6cea7a8db5f855ad4f77819f1bce93f749",
        strip_prefix = "sys-0.30.0",
        patches = [
            # releaser:patch-cmd gazelle -repo_root . -go_prefix golang.org/x/sys -go_naming_convention import_alias
            Label("//third_party:org_golang_x_sys-gazelle.patch"),
        ],
        patch_args = ["-p1"],
    )

    # Needed by golang.org/x/tools/go/packages
    # releaser:upgrade-dep golang xerrors
    wrapper(
        http_archive,
        name = "org_golang_x_xerrors",
        # master, as of 2024-05-20
        urls = [
            "https://mirror.bazel.build/github.com/golang/xerrors/archive/104605ab7028f4af38a8aff92ac848a51bd53c5d.zip",
            "https://github.com/golang/xerrors/archive/104605ab7028f4af38a8aff92ac848a51bd53c5d.zip",
        ],
        sha256 = "007a5988932222d36c106636de7f0031bb26c426327a8f1253fbf17c7c9756c1",
        strip_prefix = "xerrors-104605ab7028f4af38a8aff92ac848a51bd53c5d",
        patches = [
            # releaser:patch-cmd gazelle -repo_root . -go_prefix golang.org/x/xerrors -go_naming_convention import_alias
            Label("//third_party:org_golang_x_xerrors-gazelle.patch"),
        ],
        patch_args = ["-p1"],
    )

    # Proto dependencies
    # These are limited as much as possible. In most cases, users need to
    # declare these on their own (probably via go_repository rules generated
    # with 'gazelle update-repos -from_file=go.mod). There are several
    # reasons for this:
    #
    # * com_google_protobuf has its own dependency macro. We can't load
    #   the macro here.
    # * rules_proto also has a dependency macro. It's only needed by tests and
    #   by gogo_special_proto. Users will need to declare it anyway.
    # * org_golang_google_grpc has too many dependencies for us to maintain.
    # * In general, declaring dependencies here confuses users when they
    #   declare their own dependencies later. Bazel ignores these.
    # * Most proto repos are updated more frequently than rules_go, and
    #   we can't keep up.

    # Go protobuf runtime library and utilities.
    # releaser:upgrade-dep protocolbuffers protobuf-go
    wrapper(
        http_archive,
        name = "org_golang_google_protobuf",
        sha256 = "87fc5518c998c350c44a0feab93b236cc10b2d184d4a8b8129b8991d4d5ca584",
        # v1.36.3, from 2025-01-15
        urls = [
            "https://mirror.bazel.build/github.com/protocolbuffers/protobuf-go/archive/refs/tags/v1.36.3.zip",
            "https://github.com/protocolbuffers/protobuf-go/archive/refs/tags/v1.36.3.zip",
        ],
        strip_prefix = "protobuf-go-1.36.3",
        patches = [
            # releaser:patch-cmd gazelle -repo_root . -go_prefix google.golang.org/protobuf -go_naming_convention import_alias -proto disable_global
            Label("//third_party:org_golang_google_protobuf-gazelle.patch"),
        ],
        patch_args = ["-p1"],
    )

    # gRPC protoc plugin
    # releaser:upgrade-dep grpc grpc-go cmd/protoc-gen-go-grpc
    wrapper(
        http_archive,
        name = "org_golang_google_grpc_cmd_protoc_gen_go_grpc",
        sha256 = "1e84df03c94d1cded8e94da7a2df162463f3be4c7a94289d85c0871f14c7b8e3",
        # cmd/protoc-gen-go-grpc/v1.3.0, latest as of 2024-05-20
        urls = [
            "https://mirror.bazel.build/github.com/grpc/grpc-go/archive/refs/tags/cmd/protoc-gen-go-grpc/v1.3.0.zip",
            "https://github.com/grpc/grpc-go/archive/refs/tags/cmd/protoc-gen-go-grpc/v1.3.0.zip",
        ],
        strip_prefix = "grpc-go-cmd-protoc-gen-go-grpc-v1.3.0/cmd/protoc-gen-go-grpc",
        patches = [
            # releaser:patch-cmd gazelle -repo_root . -go_prefix google.golang.org/grpc/cmd/protoc-gen-go-grpc -go_naming_convention import_alias -proto disable_global
            Label("//third_party:org_golang_google_grpc_cmd_protoc_gen_go_grpc.patch"),
        ],
        patch_args = ["-p1"],
    )

    # Legacy protobuf compiler, runtime, and utilities.
    # We need to apply a patch to enable both go_proto_library and
    # go_library with pre-generated sources.
    # releaser:upgrade-dep golang protobuf
    wrapper(
        http_archive,
        name = "com_github_golang_protobuf",
        # v1.5.4, latest as of 2024-05-20
        urls = [
            "https://mirror.bazel.build/github.com/golang/protobuf/archive/refs/tags/v1.5.4.zip",
            "https://github.com/golang/protobuf/archive/refs/tags/v1.5.4.zip",
        ],
        sha256 = "9efeb4561ed4fbb9cefe97da407bb7b6247d4ed3dee4bfc2c24fc03dd4b5596d",
        strip_prefix = "protobuf-1.5.4",
        patches = [
            # releaser:patch-cmd gazelle -repo_root . -go_prefix github.com/golang/protobuf -go_naming_convention import_alias -proto disable_global
            Label("//third_party:com_github_golang_protobuf-gazelle.patch"),
        ],
        patch_args = ["-p1"],
    )

    # releaser:upgrade-dep gogo protobuf
    wrapper(
        http_archive,
        name = "com_github_gogo_protobuf",
        # v1.3.2, latest as of 2024-05-20
        urls = [
            "https://mirror.bazel.build/github.com/gogo/protobuf/archive/refs/tags/v1.3.2.zip",
            "https://github.com/gogo/protobuf/archive/refs/tags/v1.3.2.zip",
        ],
        sha256 = "f89f8241af909ce3226562d135c25b28e656ae173337b3e58ede917aa26e1e3c",
        strip_prefix = "protobuf-1.3.2",
        patches = [
            # releaser:patch-cmd gazelle -repo_root . -go_prefix github.com/gogo/protobuf -go_naming_convention import_alias -proto legacy
            Label("//third_party:com_github_gogo_protobuf-gazelle.patch"),
        ],
        patch_args = ["-p1"],
    )

    wrapper(
        gogo_special_proto,
        name = "gogo_special_proto",
    )

    # go_library targets with pre-generated sources for Well Known Types.
    # Doesn't belong here, but it would be an annoying source of errors if
    # this weren't generated with -proto disable_global.
    # releaser:upgrade-dep googleapis go-genproto
    wrapper(
        http_archive,
        name = "org_golang_google_genproto",
        # from 2025-01-15
        urls = [
            "https://mirror.bazel.build/github.com/googleapis/go-genproto/archive/1a7da9e5054f0b2e0ac1394d0a8538a3b74f6983.zip",
            "https://github.com/googleapis/go-genproto/archive/1a7da9e5054f0b2e0ac1394d0a8538a3b74f6983.zip",
        ],
        sha256 = "e0123b04bc8265fb288f565c4fd867af1e846a3d20306988c6d83c05736f2e5b",
        strip_prefix = "go-genproto-1a7da9e5054f0b2e0ac1394d0a8538a3b74f6983",
        patches = [
            # releaser:patch-cmd gazelle -repo_root . -go_prefix google.golang.org/genproto -go_naming_convention import_alias -proto disable_global
            Label("//third_party:org_golang_google_genproto-gazelle.patch"),
        ],
        patch_args = ["-p1"],
    )

    wrapper(
        http_archive,
        name = "platforms",
        sha256 = "218efe8ee736d26a3572663b374a253c012b716d8af0c07e842e82f238a0a7ee",
        urls = [
            "https://mirror.bazel.build/github.com/bazelbuild/platforms/releases/download/0.0.10/platforms-0.0.10.tar.gz",
            "https://github.com/bazelbuild/platforms/releases/download/0.0.10/platforms-0.0.10.tar.gz",
        ],
    )

    # releaser:upgrade-dep golang mock
    _maybe(
        http_archive,
        name = "com_github_golang_mock",
        # v1.7.0-rc.1, from 2022-05-12
        urls = [
            "https://mirror.bazel.build/github.com/golang/mock/archive/refs/tags/v1.7.0-rc.1.zip",
            "https://github.com/golang/mock/archive/refs/tags/v1.7.0-rc.1.zip",
        ],
        patches = [
            # releaser:patch-cmd gazelle -repo_root . -go_prefix github.com/golang/mock -go_naming_convention import_alias
            Label("//third_party:com_github_golang_mock-gazelle.patch"),
        ],
        patch_args = ["-p1"],
        sha256 = "5359c78b0c1649cf7beb3b48ff8b1d1aaf0243b22ea4789aba94805280075d8e",
        strip_prefix = "mock-1.7.0-rc.1",
    )

    # This may be overridden by go_register_toolchains, but it's not mandatory
    # for users to call that function (they may declare their own @go_sdk and
    # register their own toolchains).
    wrapper(
        go_register_nogo,
        name = "io_bazel_rules_nogo",
        nogo = DEFAULT_NOGO,
    )

    _maybe(
        polyfill_bazel_features,
        name = "io_bazel_rules_go_bazel_features",
    )

    _maybe(
        _go_host_compatible_s_d_k_label,
        name = "go_host_compatible_sdk_label",
    )

    wrapper(
        http_archive,
        name = "rules_shell",
        sha256 = "d8cd4a3a91fc1dc68d4c7d6b655f09def109f7186437e3f50a9b60ab436a0c53",
        strip_prefix = "rules_shell-0.3.0",
        url = "https://github.com/bazelbuild/rules_shell/releases/download/v0.3.0/rules_shell-v0.3.0.tar.gz",
    )

def _go_host_compatible_sdk_label_impl(ctx):
    ctx.file("BUILD.bazel")
    ctx.file("defs.bzl", """HOST_COMPATIBLE_SDK = Label("@go_sdk//:ROOT")""")

# This rule name has to avoid containing both "go_" and "_sdk" as substrings
# due to this check in Gazelle:
# https://github.com/bazelbuild/bazel-gazelle/blob/f08119735757370319d4f8c7653c0805fdae4817/deps.bzl#L92
_go_host_compatible_s_d_k_label = repository_rule(_go_host_compatible_sdk_label_impl)

def _maybe(repo_rule, name, **kwargs):
    if name not in native.existing_rules():
        repo_rule(name = name, **kwargs)

def _always(repo_rule, name, **kwargs):
    repo_rule(name = name, **kwargs)
