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

load(
    "//go:def.bzl",
    "GoArchive",
    "GoLibrary",
    "GoSource",
    "go_context",
)
load(
    "//proto:compiler.bzl",
    "GoProtoCompiler",
    "proto_path",
)
load(
    "//go/private:common.bzl",
    "GO_TOOLCHAIN",
)
load(
    "@rules_proto//proto:defs.bzl",
    "ProtoInfo",
)
load(
    "//go/private:providers.bzl",
    "GoProtoInfo",
)

GoProtoImports = provider()

def get_imports(target, attr, importpath):
    direct = dict()
    for src in target[ProtoInfo].check_deps_sources.to_list():
        direct["{}={}".format(proto_path(src, target[ProtoInfo]), importpath)] = True

    transitive = [
        dep[GoProtoImports].imports
        for dep in getattr(attr, "deps", [])
        if GoProtoImports in dep
    ]
    return depset(direct = direct.keys(), transitive = transitive)

def _library_to_source(_go, attr, source, merge):
    compiler = attr._compiler
    if GoSource in compiler:
        merge(source, compiler[GoSource])

def _compile_proto(ctx, target, attr):
    go = go_context(ctx)
    go_srcs = []
    proto_info = target[ProtoInfo]
    imports = get_imports(target, attr, go.importpath)
    compiler = ctx.attr._compiler[GoProtoCompiler]
    if proto_info.direct_sources:
        go_srcs.extend(compiler.compile(
            go,
            compiler = compiler,
            protos = [proto_info],
            imports = imports,
            importpath = go.importpath,
        ))

    go_deps = getattr(attr, "deps", [])
    library = go.new_library(
        go,
        resolver = _library_to_source,
        srcs = go_srcs,
        deps = go_deps + compiler.deps,
    )
    source = go.library_to_source(go, ctx.attr, library, False)
    archive = go.archive(go, source)
    return {
        "imports": GoProtoImports(
            imports = imports,
        ),
        "info": GoProtoInfo(
            library = library,
            source = source,
            archive = archive,
        ),
    }

def _go_proto_aspect_impl(target, ctx):
    providers = _compile_proto(ctx, target, ctx.rule.attr)

    return [
        providers["imports"],
        providers["info"],
    ]

_go_proto_aspect = aspect(
    _go_proto_aspect_impl,
    attrs = {
        "_compiler": attr.label(
            default = "//proto:go_proto",
        ),
        "_go_context_data": attr.label(
            default = "//:go_context_data",
        ),
    },
    attr_aspects = [
        "deps",
    ],
    required_providers = [ProtoInfo],
    provides = [GoProtoImports, GoProtoInfo],
    toolchains = [GO_TOOLCHAIN],
)

def _go_proto_library_impl(ctx):
    if len(ctx.attr.protos) != 1:
        fail("'protos' attribute must contain exactly one element. Got %s." % len(ctx.attr.protos))
    proto = ctx.attr.protos[0]

    return [
        proto[GoProtoImports],
        proto[GoProtoInfo].library,
        proto[GoProtoInfo].source,
        proto[GoProtoInfo].archive,
    ]

go_proto_library = rule(
    implementation = _go_proto_library_impl,
    attrs = {
        "protos": attr.label_list(
            providers = [ProtoInfo],
            aspects = [_go_proto_aspect],
            mandatory = True,
        ),
    },
    provides = [
        GoProtoImports,  # This is used by go_grpc_library to determine importpaths for proto deps
        GoLibrary,
        GoSource,
        GoArchive,
    ],
)

def _go_grpc_library_impl(ctx):
    if len(ctx.attr.protos) != 1:
        fail("protos attribute must be exactly one target")

    providers = _compile_proto(ctx, ctx.attr.protos[0], ctx.attr)

    return [
        providers["info"].library,
        providers["info"].source,
        providers["info"].archive,
    ]

go_grpc_library = rule(
    implementation = _go_grpc_library_impl,
    attrs = {
        "protos": attr.label_list(
            providers = [ProtoInfo],
            mandatory = True,
        ),
        "deps": attr.label_list(
            providers = [GoLibrary],
        ),
        "importpath": attr.string(),
        "_compiler": attr.label(
            default = "//proto:go_grpc",
        ),
        "_go_context_data": attr.label(
            default = "//:go_context_data",
        ),
    },
    provides = [
        GoLibrary,
        GoSource,
        GoArchive,
    ],
    toolchains = [GO_TOOLCHAIN],
)
