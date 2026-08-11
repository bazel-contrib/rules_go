// Copyright 2026 The Bazel Authors. All rights reserved.
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

package runfiles

import (
	"bufio"
	"os"
	"strings"
	"sync"
)

// packageRepoMapRlocation is the runfiles-root-relative path of a file that
// maps the link-time package path of each Go package linked into this binary
// to the canonical name of the Bazel repository containing its sources. It
// is set to a non-empty value via -X by the link action of rules_go versions
// that emit this file; it is empty if the binary was built with an older
// version of rules_go or outside of Bazel.
var packageRepoMapRlocation string

var packageRepoMap struct {
	once sync.Once
	m    map[string]string
}

// functionRepository returns the canonical name of the Bazel repository
// containing the sources of the package of the function with the given
// fully-qualified name as reported by [runtime.Frame], if known.
func functionRepository(function string) (string, bool) {
	packageRepoMap.once.Do(loadPackageRepoMap)
	return lookupFunctionRepository(function, packageRepoMap.m)
}

func lookupFunctionRepository(function string, packageRepos map[string]string) (string, bool) {
	if len(packageRepos) == 0 {
		return "", false
	}
	// Cut off the type arguments of instantiations of generic functions or
	// methods, which may themselves contain slashes and dots.
	if i := strings.IndexByte(function, '['); i >= 0 {
		function = function[:i]
	}
	// The package path is everything up to some '.' after the last '/':
	// dots before the last slash are always part of the package path (e.g.
	// in "example.com/foo/bar.F"), while dots after it separate the last
	// package path element from the (possibly nested) function and receiver
	// names. Since the last package path element may itself contain dots
	// (e.g. "gopkg.in/yaml.v2"), try successively longer candidates until
	// one matches a linked package.
	start := strings.LastIndexByte(function, '/') + 1
	for i := start; i < len(function); i++ {
		if function[i] != '.' {
			continue
		}
		if repo, ok := packageRepos[function[:i]]; ok {
			return repo, true
		}
	}
	return "", false
}

func loadPackageRepoMap() {
	if packageRepoMapRlocation == "" {
		return
	}
	// Create a fresh Runfiles instance instead of using the global one: the
	// global instance may currently be initializing on this very goroutine
	// (New calls CallerRepository, which may end up here) and its sync.Once
	// would deadlock. Passing an explicit source repository keeps New from
	// recursing into CallerRepository; the source repository is irrelevant
	// here since the path below is already runfiles-root-relative.
	r, err := New(SourceRepo(""))
	if err != nil {
		return
	}
	p, err := r.impl.path(packageRepoMapRlocation)
	if err != nil {
		return
	}
	f, err := os.Open(p)
	if err != nil {
		return
	}
	defer f.Close()

	// Each line has the form:
	// link-time package path,canonical name of the repository
	m := make(map[string]string)
	s := bufio.NewScanner(f)
	for s.Scan() {
		packagePath, repo, ok := strings.Cut(s.Text(), ",")
		if !ok {
			continue
		}
		m[packagePath] = repo
	}
	if s.Err() != nil {
		return
	}
	packageRepoMap.m = m
}
