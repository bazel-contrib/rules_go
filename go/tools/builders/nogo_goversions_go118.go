//go:build go1.18 && !go1.21
// +build go1.18,!go1.21

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

// Go 1.18 through Go 1.20 added types.Config.GoVersion, but go/types still
// only accepts the language version form go1.N there. Info.FileVersions does
// not exist yet, so the main code needs this narrower implementation.
func initGoVersionConfig(config *types.Config, goVersion string) {
	config.GoVersion = goVersion
}

// Go 1.18-go1.20 only accepts go1.N in types.Config.GoVersion.
func normalizeGoVersionForTypes(goVersion string) string {
	return trimGoPatchVersion(goVersion)
}

func initFileVersions(*types.Info) {}
