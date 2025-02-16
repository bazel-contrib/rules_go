#!/usr/bin/env python3
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

from os import path
from subprocess import check_output

POPULAR_REPOS = [
    dict(
        name = "org_golang_x_crypto",
        importpath = "golang.org/x/crypto",
        commit = "e47973b1c1089f6c67ab89261f7aa067b3d611d2",
        excludes = [
            "internal/wycheproof:wycheproof_test", # requires build cache
            "nacl/secretbox:secretbox_test", # panics in salsa2020_amd64.s
            "ssh/agent:agent_test",
            "ssh/test:test_test",
            "ssh:ssh_test",
        ],
    ),

    dict(
        name = "org_golang_x_net",
        importpath = "golang.org/x/net",
        commit = "e18ecbb051101a46fc263334b127c89bc7bff7ea",
        excludes = [
            "bpf:bpf_test", # Needs testdata directory
            "html/charset:charset_test", # Needs testdata directory
            "http2:http2_test", # Needs testdata directory
            "icmp:icmp_test", # icmp requires adjusting kernel options.
            "internal/socket:socket_test", # Needs GOROOT.
            "nettest:nettest_test", #
            "lif:lif_test",
        ],
    ),

    dict(
        name = "org_golang_x_sys",
        importpath = "golang.org/x/sys",
        commit = "390168757d9c647283340d526204e3409d5903f3",
        excludes = [
            "unix:unix_test", # TestOpenByHandleAt reads source file.
            "windows:windows_test", # Needs testdata directory
        ],
    ),

    dict(
        name = "org_golang_x_text",
        importpath = "golang.org/x/text",
        commit = "e3aa4adf54f644ca0cb35f1f1fb19b239c40ef04",
        excludes = [
            "encoding/charmap:charmap_test", # Needs testdata directory
            "encoding/japanese:japanese_test", # Needs testdata directory
            "encoding/korean:korean_test", # Needs testdata directory
            "encoding/simplifiedchinese:simplifiedchinese_test", # Needs testdata directory
            "encoding/traditionalchinese:traditionalchinese_test", # Needs testdata directory
            "encoding/unicode/utf32:utf32_test", # Needs testdata directory
            "encoding/unicode:unicode_test", # Needs testdata directory
            "internal/cldrtree:cldrtree_test", # Needs testdata directory
            "internal/gen/bitfield:bitfield_test", # Needs runfiles
            "message/pipeline/testdata/test1:test1_test", # Not a real test
            "message/pipeline:pipeline_test", # Needs testdata directory
        ],
    ),

    dict(
        name = "org_golang_x_tools",
        importpath = "golang.org/x/tools",
        commit = "fe37c9e135b934191089b245ac29325091462508",
        excludes = [
            "blog:blog_test", # Needs goldmark
            "cmd/bundle:bundle_test", # Needs testdata directory
            "cmd/callgraph/testdata/src/pkg:pkg_test", # is testdata
            "cmd/callgraph:callgraph_test", # Needs testdata directory
            "cmd/file2fuzz:file2fuzz_test", # Requires working GOROOT, uses go build
            "cmd/fiximports:fiximports_test", # requires working GOROOT, not present in CI.
            "cmd/deadcode:deadcode_test", # Needs GOROOT
            "cmd/godoc:godoc_test", # TODO(#417)
            "cmd/gonew:gonew_test", # requires build cache
            "cmd/guru/testdata/src/referrers:referrers_test", # Not a real test
            # "cmd/guru:guru_test", # Needs testdata directory
            "cmd/signature-fuzzer/fuzz-driver:fuzz-driver_test", # requires working GOROOT
            "cmd/signature-fuzzer/fuzz-runner:fuzz-runner_test", # requires working GOROOT
            "cmd/signature-fuzzer/internal/fuzz-generator:fuzz-generator_test", # requires working GOROOT
            "cmd/stringer:stringer_test", # Needs testdata directory
            "container/intsets:intsets_test", # TODO(#413): External test depends on symbols defined in internal test.
            "copyright:copyright_test", # # requires runfiles
            "go/analysis/analysistest:analysistest_test", # requires build cache
            "go/analysis/checker:checker_test", # Needs go tool
            "go/analysis/internal/analysisflags:analysisflags_test", # calls os.Exit(0) in a test
            "go/analysis/internal/checker:checker_test", # loads test package with go/packages, which probably needs go list
            "go/analysis/internal/versiontest:versiontest_test", # Needs GOROOT
            "go/analysis/multichecker:multichecker_test", # requires go vet
            "go/analysis/passes/asmdecl:asmdecl_test", # Needs testdata directory
            "go/analysis/passes/appends:appends_test", # Needs GOROOT
            "go/analysis/passes/assign:assign_test", # Needs testdata directory
            "go/analysis/passes/atomic:atomic_test", # Needs testdata directory
            "go/analysis/passes/atomicalign:atomicalign_test", # requires go list
            "go/analysis/passes/bools:bools_test", # Needs testdata directory
            "go/analysis/passes/buildssa:buildssa_test", # Needs testdata directory
            "go/analysis/passes/buildtag:buildtag_test", # Needs testdata directory
            "go/analysis/passes/cgocall:cgocall_test", # Needs testdata directory
            "go/analysis/passes/composite:composite_test", # Needs testdata directory
            "go/analysis/passes/composite/testdata/src/a:a_test", # Does not compile
            "go/analysis/passes/copylock:copylock_test", # Needs testdata directory
            "go/analysis/passes/ctrlflow:ctrlflow_test", # Needs testdata directory
            "go/analysis/passes/deepequalerrors:deepequalerrors_test", # requires go list
            "go/analysis/passes/defers:defers_test", # Needs GOROOT
            "go/analysis/passes/directive:directive_test", # Needs GOROOT
            "go/analysis/passes/errorsas:errorsas_test", # requires go list and testdata
            "go/analysis/passes/fieldalignment:fieldalignment_test", # Needs GOROOT
            "go/analysis/passes/findcall:findcall_test", # requires build cache
            "go/analysis/passes/framepointer:framepointer_test", # Needs GOROOT
            "go/analysis/passes/httpmux:httpmux_test", # Needs GOROOT
            "go/analysis/passes/httpresponse:httpresponse_test", # Needs testdata directory
            "go/analysis/passes/ifaceassert:ifaceassert_test", # Needs GOROOT
            "go/analysis/passes/loopclosure:loopclosure_test", # Needs testdata directory
            "go/analysis/passes/lostcancel:lostcancel_test", # Needs testdata directory
            "go/analysis/passes/nilfunc:nilfunc_test", # Needs testdata directory
            "go/analysis/passes/nilness:nilness_test", # Needs testdata directory
            "go/analysis/passes/pkgfact:pkgfact_test", # requires go list
            "go/analysis/passes/printf:printf_test", # Needs testdata directory
            "go/analysis/passes/reflectvaluecompare:reflectvaluecompare_test", # Needs testdata directory
            "go/analysis/passes/shadow:shadow_test", # Needs testdata directory
            "go/analysis/passes/shift:shift_test", # Needs testdata director
            "go/analysis/passes/sigchanyzer:sigchanyzer_test", # Needs testdata directory
            "go/analysis/passes/slog:slog_test", # Needs GOROOT
            "go/analysis/passes/sortslice:sortslice_test", # Needs 'go list'
            "go/analysis/passes/stdmethods:stdmethods_test", # Needs testdata directory
            "go/analysis/passes/stdversion:stdversion_test", # Needs GOROOT
            "go/analysis/passes/stringintconv:stringintconv_test", # Needs 'go list'
            "go/analysis/passes/structtag:structtag_test", # Needs testdata directory
            "go/analysis/passes/testinggoroutine:testinggoroutine_test", # Need 'go env'
            "go/analysis/passes/tests/testdata/src/a:a_test", # Not a real test
            "go/analysis/passes/tests/testdata/src/b_x_test:b_x_test_test", # Not a real test
            "go/analysis/passes/tests/testdata/src/divergent:divergent_test", # Not a real test
            "go/analysis/passes/tests/testdata/src/typeparams:typeparams_test", # Not a real test
            "go/analysis/passes/tests:tests_test", # Needs testdata directory
            "go/analysis/passes/unmarshal:unmarshal_test", # Needs go list
            "go/analysis/passes/unreachable:unreachable_test", # Needs testdata directory
            "go/analysis/passes/unsafeptr:unsafeptr_test", # Needs testdata directory
            "go/analysis/passes/unusedresult:unusedresult_test", # Needs testdata directory
            "go/analysis/passes/unusedwrite:unusedwrite_test", # Needs testdata directory
            "go/analysis/passes/timeformat:timeformat_test", # Needs go tool
            "go/analysis/passes/usesgenerics:usesgenerics_test", # Needs go tool
            "go/analysis/passes/waitgroup:waitgroup_test", # Needs go tool
            "go/analysis/unitchecker:unitchecker_test", # requires go vet
            "go/ast/inspector:inspector_test", # requires GOROOT and GOPATH
            "go/buildutil:buildutil_test", # Needs testdata directory
            "go/callgraph/cha:cha_test", # Needs testdata directory
            "go/callgraph/rta:rta_test", # Needs testdata directory
            "go/callgraph/static:static_test", # Needs go tool
            "go/callgraph/vta:vta_test", # Needs testdata directory
            "go/cfg:cfg_test", # Needs GOROOT
            "go/expect:expect_test", # Needs testdata directory
            "go/gccgoexportdata:gccgoexportdata_test", # Needs testdata directory
            "go/gcexportdata:gcexportdata_test", # Needs testdata directory
            "go/internal/gccgoimporter:gccgoimporter_test", # Needs testdata directory
            "go/loader:loader_test", # Needs testdata directory
            "go/packages/packagestest/testdata/groups/two/primarymod/expect:expect_test", # Is testdata
            "go/packages/packagestest/testdata:testdata_test", # Is testdata
            "go/packages/packagestest:packagestest_test", # requires build cache
            "go/packages:packages_test", # Hah!
            # "go/pointer:pointer_test", # Needs testdata directory
            "go/ssa/interp:interp_test", # Needs testdata directory
            "go/ssa/ssautil:ssautil_test", # Needs testdata directory
            "go/ssa:ssa_test", # Needs testdata directory
            "go/types/typeutil:typeutil_test", # requires GOROOT
            "go/types/objectpath:objectpath_test", # Incomaptible with Go SDK 1.18.3. Fixed in master but not yet released. TODO: fixme
            "godoc/static:static_test", # requires data files
            "godoc/vfs/zipfs:zipfs_test", # requires GOROOT
            "godoc:godoc_test", # requires GOROOT and GOPATH
            "internal/analysisinternal:analysisinternal_test", # requires GOROOT and GOPATH
            "internal/apidiff:apidiff_test", # Needs testdata directory
            "internal/astutil/cursor:cursor_test", # requires GOROOT
            "internal/diff/difftest:difftest_test", # Needs diff tool
            "internal/diffp:diffp_test", # Needs testdata directory
            "internal/drivertest:drivertest_test", # Needs go tool
            "internal/expect:expect_test", # Needs testdata directory
            "internal/facts:facts_test", # loads test package with go/packages, which probably needs go list
            "internal/gcimporter:gcimporter_test", # Needs testdata directory
            "internal/gocommand:gocommand_test", # Needs go tool
            "internal/imports:imports_test", # Needs testdata directory
            "internal/packagestest:packagestest_test", # Needs go tool
            "internal/packagestest/testdata/groups/two/primarymod/expect:expect_test",
            "internal/pprof:pprof_test", # Needs testdata directory
            "internal/refactor/inline:inline_test", # Needs GOROOT
            "internal/refactor/inline/analyzer:analyzer_test", # Needs GOROOT
            "internal/typeparams:typeparams_test", # Needs go tool
            "internal/testfiles:testfiles_test", # Needs testdata directory
            "internal/versions:versions_test", # Needs GoVersion
            "present:present_test", # Needs goldmark
            "refactor/eg:eg_test", # Needs testdata directory
            "refactor/importgraph:importgraph_test", # TODO(#417)
            "refactor/rename:rename_test", # TODO(#417)
        ],
        build_excludes = [
            "blog:blog", # requires present
            "cmd/deadcode:deadcode", # requires x_telemetry
            "cmd/godoc:godoc", # requires godoc
            "godoc:godoc", # requires goldmark
            "present:present", # Needs goldmark
        ],
    ),

    dict(
        name = "com_github_golang_glog",
        importpath = "github.com/golang/glog",
        commit = "23def4e6c14b4da8ac2ed8007337bc5eb5007998",
    ),

    dict(
        name = "org_golang_x_sync",
        importpath = "golang.org/x/sync",
        commit = "036812b2e83c0ddf193dd5a34e034151da389d09",
    ),

    dict(
        name = "org_golang_x_mod",
        importpath = "golang.org/x/mod",
        commit = "86c51ed26bb44749b7d60a57bab0e7524656fe8a",
        excludes = [
            "sumdb/tlog:tlog_test", # Needs network, not available on RBE
            "zip:zip_test", # Needs vcs tools, not available on RBE
        ],
    ),
  ]

COPYRIGHT_HEADER = """
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
""".strip()

BZL_HEADER = COPYRIGHT_HEADER + """

load("@bazel_gazelle//:def.bzl", "go_repository")

def _maybe(repo_rule, name, **kwargs):
    if name not in native.existing_rules():
        repo_rule(name = name, **kwargs)

def popular_repos():
"""

BUILD_HEADER = COPYRIGHT_HEADER

DOCUMENTATION_HEADER = """
Popular repository tests
========================

These tests are designed to check that gazelle and rules_go together can cope
with a list of popluar repositories people depend on.

It helps catch changes that might break a large number of users.

.. contents::

""".lstrip()

LOAD_BAZEL_TEST_RULE = """
load("@bazel_skylib//rules:build_test.bzl", "build_test")
"""

def popular_repos_bzl():
  with open(path.join(path.dirname(__file__), "popular_repos.bzl"), "w") as f:
    f.write(BZL_HEADER)
    for repo in POPULAR_REPOS:
      f.write("    _maybe(\n        go_repository,\n")
      for k in ["name", "importpath", "commit", "strip_prefix", "type", "build_file_proto_mode"]:
        if k in repo: f.write('        {} = "{}",\n'.format(k, repo[k]))
      for k in ["urls"]:
        if k in repo: f.write('        {} = ["{}"],\n'.format(k, repo[k]))
      f.write("    )\n")

def build_bazel():
  with open(path.join(path.dirname(__file__), "BUILD.bazel"), "w") as f:
    f.write(BUILD_HEADER)
    f.write("\n" + LOAD_BAZEL_TEST_RULE)
    build_only = []
    build_excludes = []
    for repo in POPULAR_REPOS:
      name = repo["name"]
      tests = check_output(["bazel", "query", "kind(go_test, \"@{}//...\")".format(name)], text=True).split("\n")
      excludes = ["@{}//{}".format(name, l) for l in repo.get("excludes", [])]
      build_excludes.extend(["@{}//{}".format(name, l) for l in repo.get("build_excludes", [])])
      for k in repo:
        if k.endswith("_excludes") or k.endswith("_tests"):
          excludes.extend(["@{}//{}".format(name, l) for l in repo[k]])
      build_only.extend(excludes)
      f.write('\ntest_suite(\n')
      f.write('    name = "{}",\n'.format(name))
      f.write('    tests = [\n')
      actual = []
      for test in sorted(tests, key=lambda test: test.replace(":", "!")):
        if test in excludes or not test: continue
        f.write('        "{}",\n'.format(test))
        actual.append(test)
      f.write('    ],\n')
      #TODO: add in the platform "select" tests
      f.write(')\n')
      repo["actual"] = actual

    # add bazel test rule
    f.write('\nbuild_test(\n')
    f.write('    name = "{}",\n'.format("build_only"))
    f.write('    targets = [\n')
    for package in build_only:
      if "/internal/" not in package and "/testdata/" not in package:
        p = package[:-5] if package.endswith("_test") else package
        if p not in build_excludes:
          f.write('        "{}",\n'.format(p))
    f.write('    ],\n')
    f.write(')\n')

def readme_rst():
  with open(path.join(path.dirname(__file__), "README.rst"), "w") as f:
    f.write(DOCUMENTATION_HEADER)
    for repo in POPULAR_REPOS:
      name = repo["name"]
      f.write("{}\n{}\n\n".format(name, "_"*len(name)))
      f.write("This runs tests from the repository `{0} <https://{0}>`_\n\n".format(repo["importpath"]))
      for test in repo["actual"]:
          f.write("* {}\n".format(test))
      f.write("\n\n")


def main():
  popular_repos_bzl()
  build_bazel()
  readme_rst()

if __name__ == "__main__":
    main()
