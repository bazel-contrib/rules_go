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

package main

import (
	"bufio"
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// compressArchive runs after all Go, assembly, and cgo objects have been packed.
func compressArchive(filename string) error {
	source, err := os.Open(filename)
	if err != nil {
		return err
	}
	defer source.Close()
	compressed, err := os.CreateTemp(filepath.Dir(filename), "archive-*.gz")
	if err != nil {
		return err
	}
	defer os.Remove(compressed.Name())
	defer compressed.Close()

	// Leave gzip header metadata empty so identical archives have identical bytes.
	writer, err := gzip.NewWriterLevel(compressed, 3)
	if err != nil {
		return err
	}
	if _, err := io.Copy(writer, source); err != nil {
		writer.Close()
		return fmt.Errorf("compressing archive %s: %w", filename, err)
	}
	if err := writer.Close(); err != nil {
		return err
	}
	if err := compressed.Close(); err != nil {
		return err
	}
	if err := source.Close(); err != nil {
		return err
	}
	return os.Rename(compressed.Name(), filename)
}

// expandArchive leaves raw archives alone, including those from reset tool
// configurations, and never modifies an input in the action's sandbox.
func expandArchive(filename, workDir string) (string, error) {
	source, err := os.Open(filename)
	if err != nil {
		return "", err
	}
	defer source.Close()
	buffered := bufio.NewReader(source)
	header, err := buffered.Peek(2)
	if err != nil && err != io.EOF {
		return "", err
	}
	if len(header) < 2 || header[0] != 0x1f || header[1] != 0x8b {
		return filename, nil
	}
	reader, err := gzip.NewReader(buffered)
	if err != nil {
		return "", fmt.Errorf("reading compressed archive %s: %w", filename, err)
	}
	defer reader.Close()
	expanded, err := os.CreateTemp(workDir, "archive-*.a")
	if err != nil {
		return "", err
	}
	_, copyErr := io.Copy(expanded, reader)
	closeErr := expanded.Close()
	if copyErr != nil {
		os.Remove(expanded.Name())
		return "", fmt.Errorf("expanding archive %s: %w", filename, copyErr)
	}
	if closeErr != nil {
		os.Remove(expanded.Name())
		return "", closeErr
	}
	return expanded.Name(), nil
}
