//go:build go1.22
// +build go1.22

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

import (
	"go/ast"
	"go/types"
)

// Go 1.22 is the first SDK where nogo can use both types.Config.GoVersion and
// types.Info.FileVersions. Keeping that logic in a version-gated file avoids
// compile-time references to newer go/types fields from older SDK builds.
func initGoVersionConfig(config *types.Config, goVersion string) {
	config.GoVersion = goVersion
}

func normalizeGoVersionForTypes(goVersion string) string {
	return goVersion
}

func initFileVersions(info *types.Info) {
	info.FileVersions = make(map[*ast.File]string)
}
