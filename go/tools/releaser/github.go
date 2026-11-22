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
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/google/go-github/v93/github"
)

type githubClient struct {
	*github.Client
}

func (gh *githubClient) listTags(ctx context.Context, org, repo string) (_ []*github.RepositoryTag, err error) {
	defer func() {
		if err != nil {
			err = fmt.Errorf("listing tags in github.com/%s/%s: %w", org, repo, err)
		}
	}()

	var allTags []*github.RepositoryTag
	err = gh.listPages(func(opts *github.ListOptions) (*github.Response, error) {
		tags, resp, err := gh.Repositories.ListTags(ctx, org, repo, opts)
		if err != nil {
			return nil, err
		}
		allTags = append(allTags, tags...)
		return resp, nil
	})
	if err != nil {
		return nil, err
	}
	return allTags, nil
}

func (gh *githubClient) listReleases(ctx context.Context, org, repo string) (_ []*github.RepositoryRelease, err error) {
	defer func() {
		if err != nil {
			err = fmt.Errorf("listing releases in github.com/%s/%s: %w", org, repo, err)
		}
	}()

	var allReleases []*github.RepositoryRelease
	err = gh.listPages(func(opts *github.ListOptions) (*github.Response, error) {
		releases, resp, err := gh.Repositories.ListReleases(ctx, org, repo, opts)
		if err != nil {
			return nil, err
		}
		allReleases = append(allReleases, releases...)
		return resp, nil
	})
	if err != nil {
		return nil, err
	}
	return allReleases, nil
}

// getReleaseByTagIncludingDraft is like
// github.RepositoriesService.GetReleaseByTag, but it also considers draft
// releases that aren't tagged yet.
func (gh *githubClient) getReleaseByTagIncludingDraft(ctx context.Context, org, repo, tag string) (*github.RepositoryRelease, error) {
	releases, err := gh.listReleases(ctx, org, repo)
	if err != nil {
		return nil, err
	}
	for _, release := range releases {
		if release.GetTagName() == tag {
			return release, nil
		}
	}
	return nil, errReleaseNotFound
}

var errReleaseNotFound = errors.New("release not found")

// githubListPages calls fn repeatedly to get all pages of a large result.
// This is useful for fetching all tags or all comments or something similar.
func (gh *githubClient) listPages(fn func(opt *github.ListOptions) (*github.Response, error)) error {
	opt := &github.ListOptions{PerPage: 50}
	for {
		resp, err := fn(opt)
		if err != nil {
			return err
		}
		if resp.NextPage == 0 {
			return nil
		}
		opt.Page = resp.NextPage
	}
}

// githubTokenFlag is used to find a GitHub personal access token on the
// command line. It accepts a raw token or a path to a file containing a token.
type githubTokenFlag string

func (f *githubTokenFlag) Set(v string) error {
	if strings.HasPrefix(v, "ghp_") {
		*(*string)(f) = v
		return nil
	}
	data, err := os.ReadFile(v)
	if err != nil {
		return fmt.Errorf("reading GitHub token: %w", err)
	}
	*(*string)(f) = string(bytes.TrimSpace(data))
	return nil
}

func (f *githubTokenFlag) String() string {
	if f == nil {
		return ""
	}
	return string(*f)
}
