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
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"strings"
	"sync"
)

var repoRootState = struct {
	once sync.Once
	dir  string
	err  error
}{}

// repoRoot returns the workspace root directory. If this program was invoked
// with 'bazel run', repoRoot returns the BUILD_WORKSPACE_DIRECTORY environment
// variable. Otherwise, repoRoot walks up the directory tree and finds a
// WORKSPACE file.
func repoRoot() (string, error) {
	repoRootState.once.Do(func() {
		if wsDir := os.Getenv("BUILD_WORKSPACE_DIRECTORY"); wsDir != "" {
			repoRootState.dir = wsDir
			return
		}
		dir, err := os.Getwd()
		if err != nil {
			repoRootState.err = err
			return
		}
		for {
			_, err := os.Stat(filepath.Join(dir, "WORKSPACE"))
			if err == nil {
				repoRootState.dir = dir
				return
			}
			if err != os.ErrNotExist {
				repoRootState.err = err
				return
			}
			parent := filepath.Dir(dir)
			if parent == dir {
				repoRootState.err = errors.New("could not find workspace directory")
				return
			}
			dir = parent
		}
	})
	return repoRootState.dir, repoRootState.err
}

// extractArchive extracts a zip or tar.gz archive opened in f, into the
// directory dir, stripping stripPrefix from each entry before extraction.
// name is the name of the archive, used for error reporting.
func extractArchive(f *os.File, name, dir, stripPrefix string) (err error) {
	if strings.HasSuffix(name, ".zip") {
		return extractZip(f, name, dir, stripPrefix)
	}
	if strings.HasSuffix(name, ".tar.gz") {
		zr, err := gzip.NewReader(f)
		if err != nil {
			return fmt.Errorf("extracting %s: %w", name, err)
		}
		defer func() {
			if cerr := zr.Close(); err == nil && cerr != nil {
				err = cerr
			}
		}()
		return extractTar(zr, name, dir, stripPrefix)
	}
	return fmt.Errorf("could not determine archive format from extension: %s", name)
}

func extractZip(zf *os.File, name, dir, stripPrefix string) (err error) {
	stripPrefix += "/"
	fi, err := zf.Stat()
	if err != nil {
		return err
	}
	defer func() {
		if err != nil {
			err = fmt.Errorf("extracting zip %s: %w", name, err)
		}
	}()

	zr, err := zip.NewReader(zf, fi.Size())
	if err != nil {
		return err
	}

	extractFile := func(f *zip.File) (err error) {
		defer func() {
			if err != nil {
				err = fmt.Errorf("extracting %s: %w", f.Name, err)
			}
		}()
		outPath, err := extractedPath(dir, stripPrefix, f.Name)
		if err != nil {
			return err
		}
		if outPath == "" {
			return nil
		}
		if strings.HasSuffix(f.Name, "/") {
			return os.MkdirAll(outPath, 0777)
		}
		r, err := f.Open()
		if err != nil {
			return err
		}
		defer r.Close()
		parent := filepath.Dir(outPath)
		if err := os.MkdirAll(parent, 0777); err != nil {
			return err
		}
		w, err := os.Create(outPath)
		if err != nil {
			return err
		}
		defer func() {
			if cerr := w.Close(); err == nil && cerr != nil {
				err = cerr
			}
		}()
		_, err = io.Copy(w, r)
		return err
	}

	for _, f := range zr.File {
		if err := extractFile(f); err != nil {
			return err
		}
	}

	return nil
}

func extractTar(r io.Reader, name, dir, stripPrefix string) (err error) {
	defer func() {
		if err != nil {
			err = fmt.Errorf("extracting tar %s: %w", name, err)
		}
	}()

	tr := tar.NewReader(r)
	extractFile := func(hdr *tar.Header) (err error) {
		outPath, err := extractedPath(dir, stripPrefix, hdr.Name)
		if err != nil {
			return err
		}
		if outPath == "" {
			return nil
		}
		switch hdr.Typeflag {
		case tar.TypeDir:
			return os.MkdirAll(outPath, 0777)
		case tar.TypeReg:
			w, err := os.Create(outPath)
			if err != nil {
				return err
			}
			defer func() {
				if cerr := w.Close(); err == nil && cerr != nil {
					err = cerr
				}
			}()
			_, err = io.Copy(w, tr)
			return err
		default:
			return fmt.Errorf("unsupported file type %x: %q", hdr.Typeflag, hdr.Name)
		}
	}

	stripPrefix += "/"
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		} else if err != nil {
			return err
		}
		if err := extractFile(hdr); err != nil {
			return err
		}
	}
	return nil
}

// extractedPath returns the file path that a file in an archive should be
// extracted to. It verifies that entryName starts with stripPrefix and does not
// point outside dir.
func extractedPath(dir, stripPrefix, entryName string) (string, error) {
	if !strings.HasPrefix(entryName, stripPrefix) {
		// Skip the file.
		return "", nil
	}
	entryName = entryName[len(stripPrefix):]
	if entryName == "" {
		return dir, nil
	}
	if path.IsAbs(entryName) {
		return "", fmt.Errorf("entry has an absolute path: %q", entryName)
	}
	if strings.HasPrefix(entryName, "../") {
		return "", fmt.Errorf("entry refers to something outside the archive: %q", entryName)
	}
	entryName = strings.TrimSuffix(entryName, "/")
	if path.Clean(entryName) != entryName {
		return "", fmt.Errorf("entry does not have a clean path: %q", entryName)
	}
	return filepath.Join(dir, entryName), nil
}

// copyDir recursively copies a directory tree.
func copyDir(toDir, fromDir string) error {
	if err := os.MkdirAll(toDir, 0777); err != nil {
		return err
	}
	return filepath.Walk(fromDir, func(path string, fi os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(fromDir, path)
		if rel == "." {
			return nil
		}
		outPath := filepath.Join(toDir, rel)
		if fi.IsDir() {
			return os.Mkdir(outPath, 0777)
		} else {
			return copyFile(outPath, path)
		}
	})
}

func copyFile(toFile, fromFile string) (err error) {
	r, err := os.Open(fromFile)
	if err != nil {
		return err
	}
	defer r.Close()
	w, err := os.Create(toFile)
	if err != nil {
		return err
	}
	defer func() {
		if cerr := w.Close(); err == nil && cerr != nil {
			err = cerr
		}
	}()
	_, err = io.Copy(w, r)
	return err
}

func sha256SumFile(name string) (string, error) {
	r, err := os.Open(name)
	if err != nil {
		return "", err
	}
	defer r.Close()
	h := sha256.New()
	if _, err := io.Copy(h, r); err != nil {
		return "", err
	}
	sum := h.Sum(nil)
	return hex.EncodeToString(sum), nil
}

// copyFileToMirror uploads a file to the GCS bucket backing mirror.bazel.build.
// gsutil must be installed, and the user must be authenticated with
// 'gcloud auth login' and be allowed to write files to the bucket.
//
// Deprecated: To mirror, please file a request to Bazel's Github Issue
func copyFileToMirror(ctx context.Context, path, fileName string) (err error) {
	return nil
}
