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

load("@io_bazel_rules_go//go/private:binary.bzl", "go_binary")
load("@io_bazel_rules_go//go/private:library.bzl", "go_library")
load("@io_bazel_rules_go//go/private:test.bzl", "go_test")
load("@io_bazel_rules_go//go/private:cgo.bzl", "setup_cgo_library")

def go_library_macro(name, srcs=None, cgo=False, cdeps=[], copts=[], clinkopts=[], **kwargs):
  cgo_object = None
  if cgo:
    cgogen = setup_cgo_library(
        name = name,
        srcs = srcs,
        cdeps = cdeps,
        copts = copts,
        clinkopts = clinkopts,
    )
    srcs = cgogen.go_srcs
    cgo_object = cgogen.cgo_object
  go_library(
      name = name,
      srcs = srcs,
      cgo_object = cgo_object,
      **kwargs
  )

def go_binary_macro(name, srcs=None, cgo=False, cdeps=[], copts=[], clinkopts=[], **kwargs):
  cgo_object = None
  if cgo:
    cgogen = setup_cgo_library(
        name = name,
        srcs = srcs,
        cdeps = cdeps,
        copts = copts,
        clinkopts = clinkopts,
    )
    srcs = cgogen.go_srcs
    cgo_object = cgogen.cgo_object
  return go_binary(
      name = name,
      srcs = srcs,
      cgo_object = cgo_object,
      **kwargs
  )

def go_test_macro(name, srcs=None, cgo=False, cdeps=[], copts=[], clinkopts=[], **kwargs):
  cgo_object = None
  if cgo:
    cgogen = setup_cgo_library(
        name = name,
        srcs = srcs,
        cdeps = cdeps,
        copts = copts,
        clinkopts = clinkopts,
    )
    srcs = cgogen.go_srcs
    cgo_object = cgogen.cgo_object
  return go_test(
      name = name,
      srcs = srcs,
      cgo_object = cgo_object,
      **kwargs
  )
