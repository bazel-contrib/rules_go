// Copyright 2019 The Bazel Authors. All rights reserved.
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

package reproducibility_test

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/bazelbuild/rules_go/go/tools/bazel_testing"
)

func TestMain(m *testing.M) {
	bazel_testing.TestMain(m, bazel_testing.Args{
		Main: `
-- BUILD.bazel --
load("@io_bazel_rules_go//go:def.bzl", "go_binary", "go_library")

go_library(
	name = "empty_lib",
	srcs = [],
	importpath = "empty_lib",
)

go_binary(
    name = "hello",
    srcs = ["hello.go"],
)

go_binary(
    name = "adder",
    srcs = [
        "adder_main.go",
        "adder.go",
        "add.c",
        "add.cpp",
        "add.h",
    ],
    cgo = True,
    linkmode = "c-archive",
)

-- hello.go --
package main

import "fmt"

func main() {
	fmt.Println("hello")
}

-- add.h --
#ifdef __cplusplus
extern "C" {
#endif

int add_c(int a, int b);
int add_cpp(int a, int b);

#ifdef __cplusplus
}
#endif

-- add.c --
#include "add.h"
#include "_cgo_export.h"

int add_c(int a, int b) { return add(a, b); }

-- add.cpp --
#include "add.h"
#include "_cgo_export.h"

int add_cpp(int a, int b) { return add(a, b); }

-- adder.go --
package main

/*
#include "add.h"
*/
import "C"

func AddC(a, b int32) int32 {
	return int32(C.add_c(C.int(a), C.int(b)))
}

func AddCPP(a, b int32) int32 {
	return int32(C.add_cpp(C.int(a), C.int(b)))
}

//export add
func add(a, b int32) int32 {
	return a + b
}

-- adder_main.go --
package main

import "fmt"

func main() {
	// Depend on some stdlib function.
	fmt.Println("In C, 2 + 2 = ", AddC(2, 2))
	fmt.Println("In C++, 2 + 2 = ", AddCPP(2, 2))
}

`,
	})
}

func Test(t *testing.T) {
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}

	// Copy the workspace to three other directories.
	// We'll run bazel commands in those directories, not here. We clean those
	// workspaces at the end of the test, but we don't want to clean this
	// directory because it's shared with other tests.
	dirs := []string{wd + "0", wd + "1", wd + "2"}
	for _, dir := range dirs {
		if err := copyTree(dir, wd); err != nil {
			t.Fatal(err)
		}
		defer func() {
			cmd := bazel_testing.BazelCmd("clean", "--expunge")
			cmd.Dir = dir
			cmd.Run()
			os.RemoveAll(dir)
		}()
	}
	defer func() {
		var wg sync.WaitGroup
		wg.Add(len(dirs))
		for _, dir := range dirs {
			go func(dir string) {
				defer wg.Done()
				cmd := bazel_testing.BazelCmd("clean", "--expunge")
				cmd.Dir = dir
				cmd.Run()
				os.RemoveAll(dir)
			}(dir)
		}
		wg.Wait()
	}()

	// Change the source file in dir2. We should detect a difference here.
	hello2Path := filepath.Join(dirs[2], "hello.go")
	hello2File, err := os.OpenFile(hello2Path, os.O_WRONLY|os.O_APPEND, 0666)
	if err != nil {
		t.Fatal(err)
	}
	defer hello2File.Close()
	if _, err := hello2File.WriteString(`func init() { fmt.Println("init") }`); err != nil {
		t.Fatal(err)
	}
	if err := hello2File.Close(); err != nil {
		t.Fatal(err)
	}

	// Build the targets in each directory.
	var wg sync.WaitGroup
	wg.Add(len(dirs))
	var errs []error
	var mu sync.Mutex
	for _, dir := range dirs {
		go func(dir string) {
			defer wg.Done()
			cmd := bazel_testing.BazelCmd("build",
				"//:all",
				"--copt=-Irelative/path/does/not/exist",
				"--linkopt=-Lrelative/path/does/not/exist",
				"@io_bazel_rules_go//go/tools/builders:go_path",
				"@go_sdk//:builder",
			)
			cmd.Dir = dir
			if err := cmd.Run(); err != nil {
				mu.Lock()
				errs = append(errs, fmt.Errorf("in %s, error running %s: %v", dir, strings.Join(cmd.Args, " "), err))
				mu.Unlock()
			}
		}(dir)
	}
	wg.Wait()
	if len(errs) > 0 {
		for _, err := range errs {
			t.Error(err)
		}
		t.Fatal("errors building")
	}

	t.Run("Check Hashes", func(t *testing.T) {
		// Hash files in each bazel-bin directory.
		dirHashes := make([][]fileHash, len(dirs))
		errs := make([]error, len(dirs))
		wg.Add(len(dirs))
		for i := range dirs {
			go func(i int) {
				defer wg.Done()
				dirHashes[i], errs[i] = hashFiles(filepath.Join(dirs[i], "bazel-out/"), func(root, path string) bool {
					// Hash everything but actions stdout/stderr and the volatile-status.txt file
					// as they are expected to change
					return strings.HasPrefix(path, filepath.Join(root, "_tmp")) ||
						path == filepath.Join(root, "volatile-status.txt")
				})
			}(i)
		}
		wg.Wait()
		for _, err := range errs {
			if err != nil {
				t.Fatal(err)
			}
		}

		// Compare dir0 and dir1. They should be identical.
		if err := compareHashes(dirHashes[0], dirHashes[1]); err != nil {
			t.Fatal(err)
		}

		// Compare dir0 and dir2. They should be different.
		if err := compareHashes(dirHashes[0], dirHashes[2]); err == nil {
			t.Fatal("dir0 and dir2 are the same")
		}
	})

	t.Run("Check builder", func(t *testing.T) {
		// Check that the go_sdk path doesn't appear in the builder binary. This path is different
		// nominally different per workspace (but in these tests, the go_sdk paths are all set to the same
		// path in WORKSPACE) -- so if this path is in the builder binary, then builds between workspaces
		// would be partially non cacheable.
		builder_file, err := os.Open(filepath.Join(dirs[0], "bazel-bin", "external", "go_sdk", "builder"))
		if err != nil {
			t.Fatal(err)
		}
		defer builder_file.Close()
		builder_data, err := ioutil.ReadAll(builder_file)
		if err != nil {
			t.Fatal(err)
		}
		if bytes.Index(builder_data, []byte("go_sdk")) != -1 {
			t.Fatalf("Found go_sdk path in builder binary, builder tool won't be reproducible")
		}
	})

	t.Run("Check stdlib", func(t *testing.T) {
		for _, dir := range dirs {
			found := false
			root, err := os.Readlink(filepath.Join(dir, "bazel-out/"))
			if err != nil {
				t.Fatal(err)
			}

			filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
				if err != nil {
					return err
				}
				if !strings.Contains(path, "stdlib_/pkg") {
					return nil
				}
				if !strings.HasSuffix(path, ".a") {
					return nil
				}
				data, err := ioutil.ReadFile(path)
				if err != nil {
					t.Fatal(err)
				}
				if bytes.Index(data, []byte("__GO_BAZEL_CC_PLACEHOLDER__")) == -1 {
					found = true
					return filepath.SkipAll
				}
				return nil
			})
			if !found {
				t.Fatal("Placeholder not found in stdlib of " + dir)
			}
		}
	})
}

func copyTree(dstRoot, srcRoot string) error {
	return filepath.Walk(srcRoot, func(srcPath string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		rel, err := filepath.Rel(srcRoot, srcPath)
		if err != nil {
			return err
		}
		var dstPath string
		if rel == "." {
			dstPath = dstRoot
		} else {
			dstPath = filepath.Join(dstRoot, rel)
		}

		if info.IsDir() {
			return os.Mkdir(dstPath, 0777)
		} else {
			return copyFile(dstPath, srcPath)
		}
	})
}

func copyFile(dstPath, srcPath string) error {
	r, err := os.Open(srcPath)
	if err != nil {
		return nil
	}
	defer r.Close()
	w, err := os.Create(dstPath)
	if err != nil {
		return err
	}
	defer w.Close()
	if _, err := io.Copy(w, r); err != nil {
		return err
	}
	return w.Close()
}

func compareHashes(lhs, rhs []fileHash) error {
	buf := &bytes.Buffer{}
	for li, ri := 0, 0; li < len(lhs) || ri < len(rhs); {
		if li < len(lhs) && (ri == len(rhs) || lhs[li].rel < rhs[ri].rel) {
			fmt.Fprintf(buf, "%s only in left\n", lhs[li].rel)
			li++
			continue
		}
		if ri < len(rhs) && (li == len(lhs) || rhs[ri].rel < lhs[li].rel) {
			fmt.Fprintf(buf, "%s only in right\n", rhs[ri].rel)
			ri++
			continue
		}
		if lhs[li].hash != rhs[ri].hash {
			fmt.Fprintf(buf, "%s is different: %s %s\n", lhs[li].rel, lhs[li].hash, rhs[ri].hash)
			if err := copyToTestOutputs(lhs[li]); err != nil {
				fmt.Fprintf(buf, "failed to copy file: %v\n", err)
			}
			if err := copyToTestOutputs(rhs[li]); err != nil {
				fmt.Fprintf(buf, "failed to copy file: %v\n", err)
			}
		}
		li++
		ri++
	}
	if errStr := buf.String(); errStr != "" {
		return errors.New(errStr)
	}
	return nil
}

type fileHash struct {
	abs, rel, hash string
}

func hashFiles(dir string, exclude func(root, path string) bool) ([]fileHash, error) {
	// Follow top-level symbolic link
	root := dir
	for {
		info, err := os.Lstat(root)
		if err != nil {
			return nil, err
		}
		if info.Mode()&os.ModeType != os.ModeSymlink {
			break
		}
		rel, err := os.Readlink(root)
		if err != nil {
			return nil, err
		}
		if filepath.IsAbs(rel) {
			root = rel
		} else {
			root = filepath.Join(filepath.Dir(dir), rel)
		}
	}

	// Gather hashes of files within the tree.
	var hashes []fileHash
	var sum [16]byte
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Skip directories and symbolic links to directories.
		if info.Mode()&os.ModeType == os.ModeSymlink {
			info, err = os.Stat(path)
			if err != nil {
				return err
			}
		}

		if info.IsDir() {
			if exclude(root, path) {
				return filepath.SkipDir
			}
			return nil
		}

		if exclude(root, path) {
			return nil
		}

		// Skip MANIFEST, runfiles_manifest, and .lo files.
		// TODO(jayconrod): find out why .lo files are not reproducible.
		base := filepath.Base(path)
		if base == "MANIFEST" || strings.HasSuffix(base, ".runfiles_manifest") || strings.HasSuffix(base, ".lo") {
			return nil
		}

		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}

		r, err := os.Open(path)
		if err != nil {
			return err
		}
		defer r.Close()
		h := sha256.New()
		if _, err := io.Copy(h, r); err != nil {
			return err
		}
		hashes = append(hashes, fileHash{abs: path, rel: rel, hash: hex.EncodeToString(h.Sum(sum[:0]))})

		return nil
	})
	if err != nil {
		return nil, err
	}
	return hashes, nil
}

// copyToTestOutputs copies a hashed file to the test outputs directory so that
// it can be inspected manually under bazel-testlogs, e.g. using diffoscope.
func copyToTestOutputs(hash fileHash) error {
	return copyFile(filepath.Join(os.Getenv("TEST_UNDECLARED_OUTPUTS_DIR"), hash.hash), hash.abs)
}
