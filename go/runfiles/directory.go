// Copyright 2020, 2021, 2021 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     https://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package runfiles

import (
	"io/fs"
	"os"
	"path"
	"path/filepath"
)

// Directory specifies the location of the runfiles directory.  You can pass
// this as an option to [New].  If unset or empty, use the value of the
// environmental variable RUNFILES_DIR.
type Directory string

func (d Directory) new(sourceRepo SourceRepo) (*Runfiles, error) {
	r := &Runfiles{
		impl: d,
		env: []string{
			directoryVar + "=" + string(d),
			legacyDirectoryVar + "=" + string(d),
		},
		sourceRepo: string(sourceRepo),
	}
	err := r.loadRepoMapping()
	return r, err
}

func (d Directory) path(s string) (string, error) {
	return filepath.Join(string(d), filepath.FromSlash(s)), nil
}

func (d Directory) open(name string) (fs.File, error) {
	dirFS := os.DirFS(string(d))
	f, err := dirFS.Open(name)
	if err != nil {
		return nil, err
	}
	return &resolvedFile{f.(*os.File), func(child string) (fs.FileInfo, error) {
		return fs.Stat(dirFS, path.Join(name, child))
	}}, nil
}

type resolvedFile struct {
	fs.ReadDirFile
	lstatChildAfterReadlink func(string) (fs.FileInfo, error)
}

func (f *resolvedFile) ReadDir(n int) ([]fs.DirEntry, error) {
	entries, err := f.ReadDirFile.ReadDir(n)
	if err != nil {
		return nil, err
	}
	for i, entry := range entries {
		// Bazel runfiles directories consist of symlinks to the real files, which may themselves
		// be directories. We want fs.WalkDir to descend into these directories as it does with the
		// manifest implementation. We do this by replacing the information about an entry that is
		// a symlink by the info of the resolved file.
		if entry.Type()&fs.ModeSymlink != 0 {
			info, err := f.lstatChildAfterReadlink(entry.Name())
			if err != nil {
				return nil, err
			}
			entries[i] = renamedDirEntry{fs.FileInfoToDirEntry(info), entry.Name()}
		}
	}
	return entries, nil
}
