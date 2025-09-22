// Copyright 2021, 2022 Google LLC
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
	"io"
	"io/fs"
	"runtime"
	"sort"
	"strings"
	"time"
)

// Open implements [fs.FS] for a [Runfiles] instance.
//
// Rlocation-style paths are supported with both apparent and canonical repo
// names. The root directory of the filesystem (".") additionally lists the
// apparent repo names that are visible to the current source repo
// (with --enable_bzlmod).
func (r *Runfiles) Open(name string) (fs.File, error) {
	// Required by testfs.TestFS.
	if !fs.ValidPath(name) || (runtime.GOOS == "windows" && strings.ContainsRune(name, '\\')) {
		return nil, &fs.PathError{Op: "open", Path: name, Err: fs.ErrInvalid}
	}
	if name == "." {
		return &rootDirFile{".", r, nil}, nil
	}
	repo, inRepoPath, hasInRepoPath := strings.Cut(name, "/")
	key := repoMappingKey{r.sourceRepo, repo}
	targetRepoDirectory, exists := r.repoMapping.Get(key)
	if !exists {
		// Either name uses a canonical repo name or refers to a root symlink.
		// In both cases, we can just open the file directly.
		return r.impl.open(name)
	}
	// Construct the path with the target repo name replaced by the canonical
	// name.
	mappedPath := targetRepoDirectory
	if hasInRepoPath {
		mappedPath += "/" + inRepoPath
	}
	f, err := r.impl.open(mappedPath)
	if err != nil {
		return nil, err
	}
	// The requested path is a child of a repo directory, return the unmodified
	// file the implementation returned.
	if hasInRepoPath {
		return f, nil
	}
	// Return a special file for a repo dir that knows its apparent name.
	return &renamedFile{f, repo}, nil
}

type rootDirFile struct {
	dirFile
	rf      *Runfiles
	entries []fs.DirEntry
}

func (r *rootDirFile) ReadDir(n int) ([]fs.DirEntry, error) {
	if err := r.initEntries(); err != nil {
		return nil, err
	}
	if n > 0 && len(r.entries) == 0 {
		return nil, io.EOF
	}
	if n <= 0 || n > len(r.entries) {
		n = len(r.entries)
	}
	entries := r.entries[:n]
	r.entries = r.entries[n:]
	return entries, nil
}

func (r *rootDirFile) initEntries() error {
	if r.entries != nil {
		return nil
	}
	// The entries of the root dir should be the apparent names of the repos
	// visible to the main repo (plus root symlinks). We thus need to read
	// the real entries and then transform and filter them.
	canonicalToApparentName := make(map[string]string)
	r.rf.repoMapping.ForEachVisible(r.rf.sourceRepo, func(targetApparentName, targetRepoDirectory string) {
		canonicalToApparentName[targetRepoDirectory] = targetApparentName
	})
	rootFile, err := r.rf.impl.open(".")
	if err != nil {
		return err
	}
	realDirFile := rootFile.(fs.ReadDirFile)
	realEntries, err := realDirFile.ReadDir(0)
	if err != nil {
		return err
	}
	for _, e := range realEntries {
		r.entries = append(r.entries, e)
		if apparent, ok := canonicalToApparentName[e.Name()]; ok && e.IsDir() && apparent != e.Name() {
			// A repo directory that is visible to the current source repo is additionally
			// materialized under its apparent name. We do not use a symlink as
			// fs.WalkDir doesn't descend into symlinks.
			r.entries = append(r.entries, renamedDirEntry{e, apparent})
		}
	}
	sort.Slice(r.entries, func(i, j int) bool {
		return r.entries[i].Name() < r.entries[j].Name()
	})
	return nil
}

type renamedFile struct {
	fs.File
	name string
}

func (r renamedFile) Stat() (fs.FileInfo, error) {
	info, err := r.File.Stat()
	if err != nil {
		return nil, err
	}
	return renamedFileInfo{info, r.name}, nil
}

func (r renamedFile) ReadDir(n int) ([]fs.DirEntry, error) {
	readDirFile, ok := r.File.(fs.ReadDirFile)
	if !ok {
		return nil, &fs.PathError{Op: "readdir", Path: r.name, Err: fs.ErrInvalid}
	}
	return readDirFile.ReadDir(n)
}

type renamedDirEntry struct {
	fs.DirEntry
	name string
}

func (r renamedDirEntry) Name() string { return r.name }
func (r renamedDirEntry) Info() (fs.FileInfo, error) {
	info, err := r.DirEntry.Info()
	if err != nil {
		return nil, err
	}
	return renamedFileInfo{info, r.name}, nil
}
func (r renamedDirEntry) String() string { return fs.FormatDirEntry(r) }

type renamedFileInfo struct {
	fs.FileInfo
	name string
}

func (r renamedFileInfo) Name() string   { return r.name }
func (r renamedFileInfo) String() string { return fs.FormatFileInfo(r) }

type emptyFile string

func (f emptyFile) Stat() (fs.FileInfo, error) { return emptyFileInfo(f), nil }
func (f emptyFile) Read([]byte) (int, error)   { return 0, io.EOF }
func (emptyFile) Close() error                 { return nil }

type emptyFileInfo string

func (i emptyFileInfo) Name() string     { return string(i) }
func (emptyFileInfo) Size() int64        { return 0 }
func (emptyFileInfo) Mode() fs.FileMode  { return 0444 }
func (emptyFileInfo) ModTime() time.Time { return time.Time{} }
func (emptyFileInfo) IsDir() bool        { return false }
func (emptyFileInfo) Sys() interface{}   { return nil }
func (i emptyFileInfo) String() string   { return fs.FormatFileInfo(i) }
