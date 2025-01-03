// Copyright 2021 The Bazel Authors. All rights reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//    http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package main

import (
	"context"
	"fmt"
	"go/build"
	"os"
	"os/signal"
	"path"
	"path/filepath"

	"golang.org/x/tools/go/packages"
)

func getenvDefault(key, defaultValue string) string {
	if v, ok := os.LookupEnv(key); ok {
		return v
	}
	return defaultValue
}

func concatStringsArrays(values ...[]string) []string {
	ret := []string{}
	for _, v := range values {
		ret = append(ret, v...)
	}
	return ret
}

func ensureAbsolutePathFromWorkspace(path string) string {
	if filepath.IsAbs(path) {
		return path
	}
	return filepath.Join(workspaceRoot, path)
}

func signalContext(parentCtx context.Context, signals ...os.Signal) (ctx context.Context, stop context.CancelFunc) {
	ctx, cancel := context.WithCancel(parentCtx)
	ch := make(chan os.Signal, 1)
	go func() {
		select {
		case <-ch:
			cancel()
		case <-ctx.Done():
		}
	}()
	signal.Notify(ch, signals...)

	return ctx, cancel
}

func isLocalPattern(pattern string) bool {
	return build.IsLocalImport(pattern) || filepath.IsAbs(pattern)
}

func packageID(pattern string) string {
	pattern = path.Clean(pattern)
	if filepath.IsAbs(pattern) {
		if relPath, err := filepath.Rel(workspaceRoot, pattern); err == nil {
			pattern = relPath
		}
	}

	return fmt.Sprintf("//%s", pattern)
}

func findPackageByID(packages []*packages.Package, id string) *packages.Package {
	for _, pkg := range packages {
		if pkg.ID == id {
			return pkg
		}
	}
	return nil
}

// get map keys
func keysFromMap[K comparable, V any](m map[K]V) []K {
	keys := make([]K, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}

// contains checks if a slice contains an element
func contains[S ~[]E, E comparable](set S, element E) bool {
	found := false
	for _, setElement := range set {
		if setElement == element {
			found = true
			break
		}
	}
	return found
}

// containsAll checks if a slice contains all elements of another slice
func containsAll[S ~[]E, E comparable](set S, subset S) bool {
	for _, subsetElement := range subset {
		if !contains(set, subsetElement) {
			return false
		}
	}
	return true
}

// equalSets checks if two slices are equal sets
func equalSets[S ~[]E, E comparable](set1 S, set2 S) bool {
	if len(set1) != len(set2) {
		return false
	}
	return containsAll(set1, set2)
}
