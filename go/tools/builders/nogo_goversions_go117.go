//go:build !go1.18
// +build !go1.18

/* Copyright 2026 The Bazel Authors. All rights reserved.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

   http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package main

import "go/types"

// These helpers are split by SDK version so nogo_main.go can call them
// unconditionally without referencing go/types APIs that do not exist yet.
// Go 1.17 and older do not expose Config.GoVersion or Info.FileVersions, so
// this variant is intentionally all no-ops.
func initGoVersionConfig(*types.Config, string) {}

func normalizeGoVersionForTypes(goVersion string) string {
	return goVersion
}

func initFileVersions(*types.Info) {}
