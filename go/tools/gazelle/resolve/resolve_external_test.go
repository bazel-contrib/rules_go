/* Copyright 2016 The Bazel Authors. All rights reserved.

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

package resolve

import (
	"fmt"
	"reflect"
	"strings"
	"testing"

	"golang.org/x/tools/go/vcs"
)

func TestSpecialCases(t *testing.T) {
	r := newStubExternalResolver()
	for _, c := range []struct {
		in, want  string
		wantError bool
	}{
		{in: "golang.org/x/net/context", want: "golang.org/x/net"},
		{in: "golang.org/x/tools/go/vcs", want: "golang.org/x/tools"},
		{in: "golang.org/x/goimports", want: "golang.org/x/goimports"},
		{in: "cloud.google.com/fashion/industry", want: "cloud.google.com/fashion"},
		{in: "github.com/foo", wantError: true},
		{in: "github.com/foo/bar", want: "github.com/foo/bar"},
		{in: "github.com/foo/bar/baz", want: "github.com/foo/bar"},
		{in: "unsupported.org/x/net/context", wantError: true},
	} {
		if got, err := r.lookupPrefix(c.in); err != nil {
			if !c.wantError {
				t.Errorf("unexpected error: %v", err)
			}
		} else if c.wantError {
			t.Errorf("unexpected success: %v", c.in)
		} else if got != c.want {
			t.Errorf("specialCases(%q) = %q; want %q", c.in, got, c.want)
		}
	}
}

func TestExternalResolver(t *testing.T) {
	r := newStubExternalResolver()
	for _, spec := range []struct {
		importpath string
		want       Label
	}{
		{
			importpath: "example.com/repo",
			want: Label{
				Repo: "com_example_repo",
				Name: DefaultLibName,
			},
		},
		{
			importpath: "example.com/repo/lib",
			want: Label{
				Repo: "com_example_repo",
				Pkg:  "lib",
				Name: DefaultLibName,
			},
		},
		{
			importpath: "example.com/repo.git/lib",
			want: Label{
				Repo: "com_example_repo_git",
				Pkg:  "lib",
				Name: DefaultLibName,
			},
		},
		{
			importpath: "example.com/lib",
			want: Label{
				Repo: "com_example",
				Pkg:  "lib",
				Name: DefaultLibName,
			},
		},
	} {
		l, err := r.Resolve(spec.importpath, "some/package")
		if err != nil {
			t.Errorf("r.Resolve(%q) failed with %v; want success", spec.importpath, err)
			continue
		}
		if got, want := l, spec.want; !reflect.DeepEqual(got, want) {
			t.Errorf("r.Resolve(%q) = %s; want %s", spec.importpath, got, want)
		}
	}
}

func newStubExternalResolver() *externalResolver {
	r := newExternalResolver()
	r.repoRootForImportPath = stubRepoRootForImportPath
	return r
}

// stubRepoRootForImportPath is a stub implementation of vcs.RepoRootForImportPath
func stubRepoRootForImportPath(importpath string, verbose bool) (*vcs.RepoRoot, error) {
	if strings.HasPrefix(importpath, "example.com/repo.git") {
		return &vcs.RepoRoot{
			VCS:  vcs.ByCmd("git"),
			Repo: "https://example.com/repo.git",
			Root: "example.com/repo.git",
		}, nil
	}

	if strings.HasPrefix(importpath, "example.com/repo") {
		return &vcs.RepoRoot{
			VCS:  vcs.ByCmd("git"),
			Repo: "https://example.com/repo.git",
			Root: "example.com/repo",
		}, nil
	}

	if strings.HasPrefix(importpath, "example.com") {
		return &vcs.RepoRoot{
			VCS:  vcs.ByCmd("git"),
			Repo: "https://example.com",
			Root: "example.com",
		}, nil
	}

	return nil, fmt.Errorf("could not resolve import path: %q", importpath)
}
