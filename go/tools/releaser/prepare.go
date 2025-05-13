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
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/google/go-github/v36/github"
	"golang.org/x/mod/semver"
	"golang.org/x/oauth2"
)

var prepareCmd = command{
	name:        "prepare",
	description: "prepares a GitHub release with notes and attached archive",
	help: `prepare -rnotes=file -version=version -githubtoken=token [-mirror]

'prepare' performs most tasks related to a rules_go release. It does everything
except publishing and tagging the release, which must be done manually,
with review. Specifically, prepare does the following:

* Creates the release branch if it doesn't exist locally. Release branches
  have names like "release-X.Y" where X and Y are the major and minor version
  numbers.
* Creates an archive zip file from the tip of the local release branch.
* Creates or updates a draft GitHub release with the given release notes.
  http_archive boilerplate is generated and appended to the release notes.
* Uploads and attaches the release archive to the GitHub release.
* Uploads the release archive to mirror.bazel.build. If the file already exists,
  it may be manually removed with 'gsutil rm gs://bazel-mirror/<github-url>'
  or manually updated with 'gsutil cp <file> gs://bazel-mirror/<github-url>'.
  This step may be skipped by setting -mirror=false.

After these steps are completed successfully, 'prepare' prompts the user to
check that CI passes, then review and publish the release.

Note that 'prepare' does not update boilerplate in WORKSPACE or README.rst for
either rules_go or Gazelle.
`,
}

func init() {
	// break init cycle
	prepareCmd.run = runPrepare
}

func runPrepare(ctx context.Context, stderr io.Writer, args []string) error {
	// Parse arguments.
	flags := flag.NewFlagSet("releaser prepare", flag.ContinueOnError)
	var rnotesPath, version string
	var githubToken githubTokenFlag
	var uploadToMirror bool
	flags.Var(&githubToken, "githubtoken", "GitHub personal access token or path to a file containing it")
	flags.BoolVar(&uploadToMirror, "mirror", false, "whether to upload dependency archives to mirror.bazel.build")
	flags.StringVar(&rnotesPath, "rnotes", "", "Name of file containing release notes in Markdown")
	flags.StringVar(&version, "version", "", "Version to release")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if flags.NArg() > 0 {
		return usageErrorf(&prepareCmd, "No arguments expected")
	}
	if githubToken == "" {
		return usageErrorf(&prepareCmd, "-githubtoken must be set")
	}
	if rnotesPath == "" {
		return usageErrorf(&prepareCmd, "-rnotes must be set")
	}
	if version == "" {
		return usageErrorf(&prepareCmd, "-version must be set")
	}
	if semver.Canonical(version) != version || semver.Build(version) != "" {
		return usageErrorf(&prepareCmd, "-version must be a canonical version, like v1.2.3")
	}

	ts := oauth2.StaticTokenSource(&oauth2.Token{AccessToken: string(githubToken)})
	tc := oauth2.NewClient(ctx, ts)
	gh := &githubClient{Client: github.NewClient(tc)}

	// Get the GitHub release.
	fmt.Fprintf(stderr, "checking if release %s exists...\n", version)
	release, err := gh.getReleaseByTagIncludingDraft(ctx, "bazel-contrib", "rules_go", version)
	if err != nil && !errors.Is(err, errReleaseNotFound) {
		return err
	}
	if release != nil && !release.GetDraft() {
		return fmt.Errorf("release %s was already published", version)
	}

	// If this is a minor release (x.y.0), create the release branch if it
	// does not exist.
	fmt.Fprintf(stderr, "verifying release branch...\n")
	rootDir, err := repoRoot()
	if err != nil {
		return err
	}
	if err := checkNoGitChanges(ctx, rootDir); err != nil {
		return err
	}
	majorMinor := semver.MajorMinor(version)
	isMinorRelease := semver.Canonical(majorMinor) == version
	branchName := "release-" + majorMinor[len("v"):]
	if !gitBranchExists(ctx, rootDir, branchName) {
		if !isMinorRelease {
			return fmt.Errorf("release branch %q does not exist locally. Fetch it, add commits, and run this command again.", branchName)
		}
		fmt.Fprintf(stderr, "creating branch %s...\n", branchName)
		if err := gitCreateBranch(ctx, rootDir, branchName, "HEAD"); err != nil {
			return err
		}
	}

	// Create an archive.
	fmt.Fprintf(stderr, "creating archive...\n")
	arcFile, err := os.CreateTemp("", "rules_go-%s-*.zip")
	if err != nil {
		return err
	}
	arcName := arcFile.Name()
	arcFile.Close()
	defer func() {
		if rerr := os.Remove(arcName); err == nil && rerr != nil {
			err = rerr
		}
	}()
	if err := gitCreateArchive(ctx, rootDir, branchName, arcName); err != nil {
		return err
	}
	arcSum, err := sha256SumFile(arcName)
	if err != nil {
		return err
	}

	// Read release notes, append boilerplate.
	rnotesData, err := os.ReadFile(rnotesPath)
	if err != nil {
		return err
	}
	rnotesData = bytes.TrimSpace(rnotesData)
	goVersion, err := findLatestGoVersion()
	if err != nil {
		return err
	}
	rnotesStr := genBoilerplate(version, arcSum, goVersion, string(rnotesData))

	// Push the release branch.
	fmt.Fprintf(stderr, "pushing branch %s to origin...\n", branchName)
	if err := gitPushBranch(ctx, rootDir, branchName); err != nil {
		return err
	}

	// Upload to mirror.bazel.build.
	arcGHURLWithoutScheme := fmt.Sprintf("github.com/bazel-contrib/rules_go/releases/download/%[1]s/rules_go-%[1]s.zip", version)
	if uploadToMirror {
		fmt.Fprintf(stderr, "uploading archive to mirror.bazel.build...\n")
		if err := copyFileToMirror(ctx, arcGHURLWithoutScheme, arcName); err != nil {
			return err
		}
	}

	// Create or update the GitHub release.
	if release == nil {
		fmt.Fprintf(stderr, "creating draft release...\n")
		draft := true
		release = &github.RepositoryRelease{
			TagName:         &version,
			TargetCommitish: &branchName,
			Name:            &version,
			Body:            &rnotesStr,
			Draft:           &draft,
		}
		if release, _, err = gh.Repositories.CreateRelease(ctx, "bazel-contrib", "rules_go", release); err != nil {
			return err
		}
	} else {
		fmt.Fprintf(stderr, "updating release...\n")
		release.Body = &rnotesStr
		if release, _, err = gh.Repositories.EditRelease(ctx, "bazel-contrib", "rules_go", release.GetID(), release); err != nil {
			return err
		}
		for _, asset := range release.Assets {
			if _, err := gh.Repositories.DeleteReleaseAsset(ctx, "bazel-contrib", "rules_go", asset.GetID()); err != nil {
				return err
			}
		}
	}
	arcFile, err = os.Open(arcName)
	if err != nil {
		return err
	}
	defer arcFile.Close()
	uploadOpts := &github.UploadOptions{
		Name:      "rules_go-" + version + ".zip",
		MediaType: "application/zip",
	}
	if _, _, err := gh.Repositories.UploadReleaseAsset(ctx, "bazel-contrib", "rules_go", release.GetID(), uploadOpts, arcFile); err != nil {
		return err
	}

	testURL := fmt.Sprintf("https://buildkite.com/bazel/rules-go-golang/builds?branch=%s", branchName)
	fmt.Fprintf(stderr, `
Release %s has been prepared and uploaded.

* Ensure that all tests pass in CI at %s.
* Review and publish the release at %s.
`, version, testURL, release.GetHTMLURL())

	return nil
}
