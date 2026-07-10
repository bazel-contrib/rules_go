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

package data_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestVisibleRunfiles(t *testing.T) {
	var got []string
	err := filepath.Walk(".", func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if base := filepath.Base(path); info.IsDir() || base == "data_test" || base == "data_test.exe" || strings.HasSuffix(base, ".package_repo_map") {
			return nil
		}
		got = append(got, filepath.ToSlash(path))
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}

	gotStr := strings.Join(got, "\n")
	wantStr := "x\ny\nz"
	if gotStr != wantStr {
		t.Errorf("got:\n%s\nwant:\n%s\n", gotStr, wantStr)
	}
}
