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

"""Public definitions for Go rules.

All public Go rules, providers, and other definitions are imported and
re-exported in this file. This allows the real location of definitions
to change for easier maintenance.

Definitions outside this file are private unless otherwise noted, and
may change without notice.
"""

load(
    "//extras:gomock.bzl",
    _gomock = "gomock",
)
load(
    "//go/private:context.bzl",
    _go_context = "go_context",
    _new_go_info = "new_go_info",
)
load(
    "//go/private:go_toolchain.bzl",
    _go_toolchain = "go_toolchain",
)
load(
    "//go/private:providers.bzl",
    _GoArchive = "GoArchive",
    _GoArchiveData = "GoArchiveData",
    _GoInfo = "GoInfo",
    _GoPath = "GoPath",
    _GoSDK = "GoSDK",
)
load(
    "//go/private/rules:cross.bzl",
    _go_cross_binary = "go_cross_binary",
)
load(
    "//go/private/rules:library.bzl",
    _go_tool_library = "go_tool_library",
)
load(
    "//go/private/rules:nogo.bzl",
    _nogo = "nogo_wrapper",
)
load(
    "//go/private/rules:sdk.bzl",
    _go_sdk = "go_sdk",
)
load(
    "//go/private/rules:source.bzl",
    _go_source = "go_source",
)
load(
    "//go/private/rules:transition.bzl",
    _go_reset_target = "go_reset_target",
)
load(
    "//go/private/rules:wrappers.bzl",
    _go_binary_macro = "go_binary_macro",
    _go_library_macro = "go_library_macro",
    _go_test_macro = "go_test_macro",
)
load(
    "//go/private/tools:path.bzl",
    _go_path = "go_path",
)

_TOOLS_NOGO = [
    "@org_golang_x_tools//go/analysis/passes/asmdecl:go_default_library",
    "@org_golang_x_tools//go/analysis/passes/assign:go_default_library",
    "@org_golang_x_tools//go/analysis/passes/atomic:go_default_library",
    "@org_golang_x_tools//go/analysis/passes/atomicalign:go_default_library",
    "@org_golang_x_tools//go/analysis/passes/bools:go_default_library",
    "@org_golang_x_tools//go/analysis/passes/buildssa:go_default_library",
    "@org_golang_x_tools//go/analysis/passes/buildtag:go_default_library",
    # TODO(#2396): pass raw cgo sources to cgocall and re-enable.
    # "@org_golang_x_tools//go/analysis/passes/cgocall:go_default_library",
    "@org_golang_x_tools//go/analysis/passes/composite:go_default_library",
    "@org_golang_x_tools//go/analysis/passes/copylock:go_default_library",
    "@org_golang_x_tools//go/analysis/passes/ctrlflow:go_default_library",
    "@org_golang_x_tools//go/analysis/passes/deepequalerrors:go_default_library",
    "@org_golang_x_tools//go/analysis/passes/errorsas:go_default_library",
    "@org_golang_x_tools//go/analysis/passes/findcall:go_default_library",
    "@org_golang_x_tools//go/analysis/passes/httpresponse:go_default_library",
    "@org_golang_x_tools//go/analysis/passes/ifaceassert:go_default_library",
    "@org_golang_x_tools//go/analysis/passes/inspect:go_default_library",
    "@org_golang_x_tools//go/analysis/passes/loopclosure:go_default_library",
    "@org_golang_x_tools//go/analysis/passes/lostcancel:go_default_library",
    "@org_golang_x_tools//go/analysis/passes/nilfunc:go_default_library",
    "@org_golang_x_tools//go/analysis/passes/nilness:go_default_library",
    "@org_golang_x_tools//go/analysis/passes/pkgfact:go_default_library",
    "@org_golang_x_tools//go/analysis/passes/printf:go_default_library",
    "@org_golang_x_tools//go/analysis/passes/shadow:go_default_library",
    "@org_golang_x_tools//go/analysis/passes/shift:go_default_library",
    "@org_golang_x_tools//go/analysis/passes/sortslice:go_default_library",
    "@org_golang_x_tools//go/analysis/passes/stdmethods:go_default_library",
    "@org_golang_x_tools//go/analysis/passes/stringintconv:go_default_library",
    "@org_golang_x_tools//go/analysis/passes/structtag:go_default_library",
    "@org_golang_x_tools//go/analysis/passes/testinggoroutine:go_default_library",
    "@org_golang_x_tools//go/analysis/passes/tests:go_default_library",
    "@org_golang_x_tools//go/analysis/passes/unmarshal:go_default_library",
    "@org_golang_x_tools//go/analysis/passes/unreachable:go_default_library",
    "@org_golang_x_tools//go/analysis/passes/unsafeptr:go_default_library",
    "@org_golang_x_tools//go/analysis/passes/unusedresult:go_default_library",
]

# TOOLS_NOGO is a list of all analysis passes in
# golang.org/x/tools/go/analysis/passes.
# This is not backward compatible, so use caution when depending on this --
# new analyses may discover issues in existing builds.
TOOLS_NOGO = [str(Label(l)) for l in _TOOLS_NOGO]

# Current version or next version to be tagged. Gazelle and other tools may
# check this to determine compatibility.
RULES_GO_VERSION = "0.50.1"

go_context = _go_context
gomock = _gomock
go_sdk = _go_sdk
go_tool_library = _go_tool_library
go_toolchain = _go_toolchain
nogo = _nogo

# This provider is deprecated and will be removed in a future release.
# Use GoInfo instead.
GoLibrary = _GoInfo

# This provider is deprecated and will be removed in a future release.
# Use GoInfo instead.
GoSource = _GoInfo

# See go/providers.rst#GoInfo for full documentation.
GoInfo = _GoInfo

# See go/toolchains.rst#new_go_info for full documentation.
new_go_info = _new_go_info

# See go/providers.rst#GoPath for full documentation.
GoPath = _GoPath

# See go/providers.rst#GoArchive for full documentation.
GoArchive = _GoArchive

# See go/providers.rst#GoArchiveData for full documentation.
GoArchiveData = _GoArchiveData

# See go/providers.rst#GoSDK for full documentation.
GoSDK = _GoSDK

# See docs/go/core/rules.md#go_library for full documentation.
go_library = _go_library_macro

# See docs/go/core/rules.md#go_binary for full documentation.
go_binary = _go_binary_macro

# See docs/go/core/rules.md#go_test for full documentation.
go_test = _go_test_macro

# See docs/go/core/rules.md#go_test for full documentation.
go_source = _go_source

# See docs/go/core/rules.md#go_path for full documentation.
go_path = _go_path

# See docs/go/core/rules.md#go_reset_target for full documentation.
go_reset_target = _go_reset_target

# See docs/go/core/rules.md#go_cross_binary for full documentation.
go_cross_binary = _go_cross_binary

def go_vet_test(*_args, **_kwargs):
    fail("The go_vet_test rule has been removed. Please migrate to nogo instead, which supports vet tests.")

def go_rule(**_kwargs):
    fail("The go_rule function has been removed. Use rule directly instead. See https://github.com/bazelbuild/rules_go/blob/master/go/toolchains.rst#writing-new-go-rules")

def go_rules_dependencies():
    _moved("go_rules_dependencies")

def go_register_toolchains(**_kwargs):
    _moved("go_register_toolchains")

def go_download_sdk(**_kwargs):
    _moved("go_download_sdk")

def go_host_sdk(**_kwargs):
    _moved("go_host_sdk")

def go_local_sdk(**_kwargs):
    _moved("go_local_sdk")

def go_wrap_sdk(**_kwargs):
    _moved("go_wrap_sdK")

def _moved(name):
    fail(name + " has moved. Please load from " +
         " @io_bazel_rules_go//go:deps.bzl instead of def.bzl.")
