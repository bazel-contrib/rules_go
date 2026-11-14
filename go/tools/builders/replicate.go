// Copyright 2018 The Bazel Authors. All rights reserved.
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

// stdlib builds the standard library in the appropriate mode into a new goroot.
package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
)

type replicateMode int

const (
	copyMode replicateMode = iota
	hardlinkMode
	softlinkMode
)

type replicateOption func(*replicateConfig)
type replicateConfig struct {
	removeFirst bool
	fileMode    replicateMode
	dirMode     replicateMode
	paths       []string
}

func replicatePaths(paths ...string) replicateOption {
	return func(config *replicateConfig) {
		config.paths = append(config.paths, paths...)
	}
}

// replicateSDK copies only declared sources: the physical SDK may contain files
// that aren't inputs to the action when the builder runs outside a sandbox.
func replicateSDK(root, dst string, srcs []string) error {
	if err := replicate(root, dst, replicatePaths("pkg/tool", "pkg/include")); err != nil {
		return err
	}
	source := filepath.Join(root, "src")
	target := filepath.Join(dst, "src")
	if err := os.RemoveAll(target); err != nil {
		return err
	}
	for _, src := range srcs {
		rel, err := filepath.Rel(source, abs(src))
		if err != nil {
			return err
		}
		if !filepath.IsLocal(rel) {
			return fmt.Errorf("SDK source %q is outside %q", src, source)
		}
		if err := replicateFile(src, filepath.Join(target, rel), &replicateConfig{}); err != nil {
			return err
		}
	}
	return nil
}

// replicatePrepare is the common preparation steps for a replication entry
func replicatePrepare(dst string, config *replicateConfig) error {
	dir := filepath.Dir(dst)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("Failed to make %s: %v", dir, err)
	}
	if config.removeFirst {
		_ = os.Remove(dst)
	}
	return nil
}

// replicateFile is called internally by replicate to map a single file from src into dst.
func replicateFile(src, dst string, config *replicateConfig) error {
	if err := replicatePrepare(dst, config); err != nil {
		return err
	}
	switch config.fileMode {
	case copyMode:
		in, err := os.Open(src)
		if err != nil {
			return err
		}
		defer in.Close()
		out, err := os.Create(dst)
		if err != nil {
			return err
		}
		_, err = io.Copy(out, in)
		closeerr := out.Close()
		if err != nil {
			return err
		}
		if closeerr != nil {
			return closeerr
		}
		s, err := os.Stat(src)
		if err != nil {
			return err
		}
		if err := os.Chmod(dst, s.Mode()); err != nil {
			return err
		}
		return nil
	case hardlinkMode:
		return os.Link(src, dst)
	case softlinkMode:
		return os.Symlink(src, dst)
	default:
		return fmt.Errorf("Invalid replication mode %d", config.fileMode)
	}
}

// replicateDir makes a tree of files visible in a new location.
// It is allowed to take any efficient method of doing so.
func replicateDir(src, dst string, config *replicateConfig) error {
	if err := replicatePrepare(dst, config); err != nil {
		return err
	}
	switch config.dirMode {
	case copyMode:
		return filepath.Walk(src, func(path string, f os.FileInfo, err error) error {
			if f.IsDir() {
				return nil
			}
			relative, err := filepath.Rel(src, path)
			if err != nil {
				return err
			}
			return replicateFile(path, filepath.Join(dst, relative), config)
		})
	case hardlinkMode:
		return os.Link(src, dst)
	case softlinkMode:
		return os.Symlink(src, dst)
	default:
		return fmt.Errorf("Invalid replication mode %d", config.fileMode)
	}
}

// replicateTree is called for each single src dst pair.
func replicateTree(src, dst string, config *replicateConfig) error {
	if err := os.RemoveAll(dst); err != nil {
		return fmt.Errorf("Failed to remove file at destination %s: %v", dst, err)
	}
	if l, err := filepath.EvalSymlinks(src); err != nil {
		return err
	} else {
		src = l
	}
	if s, err := os.Stat(src); err != nil {
		return err
	} else if s.IsDir() {
		return replicateDir(src, dst, config)
	}
	return replicateFile(src, dst, config)
}

// replicate makes a tree of files visible in a new location.
// You control how it does so using options, by default it presumes the entire tree
// of files rooted at src must be visible at dst, and that it should do so by copying.
// src is allowed to be a file, in which case just the one file is copied.
func replicate(src, dst string, options ...replicateOption) error {
	config := replicateConfig{
		removeFirst: true,
	}
	for _, option := range options {
		option(&config)
	}
	if len(config.paths) == 0 {
		return replicateTree(src, dst, &config)
	}
	for _, base := range config.paths {
		from := filepath.Join(src, base)
		to := filepath.Join(dst, base)
		if err := replicateTree(from, to, &config); err != nil {
			return err
		}
	}
	return nil
}
