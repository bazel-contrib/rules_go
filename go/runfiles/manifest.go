// Copyright 2020, 2021 Google LLC
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
	"bufio"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"syscall"
	"time"
)

// ManifestFile specifies the location of the runfile manifest file.  You can
// pass this as an option to New.  If unset or empty, use the value of the
// environmental variable RUNFILES_MANIFEST_FILE.
type ManifestFile string

func (f ManifestFile) new(sourceRepo SourceRepo) (*Runfiles, error) {
	m, err := f.parse()
	if err != nil {
		return nil, err
	}
	env := []string{
		manifestFileVar + "=" + string(f),
	}
	// Certain tools (e.g., Java tools) may need the runfiles directory, so try to find it even if
	// running with a manifest file.
	if strings.HasSuffix(string(f), ".runfiles_manifest") ||
		strings.HasSuffix(string(f), "/MANIFEST") ||
		strings.HasSuffix(string(f), "\\MANIFEST") {
		// Cut off either "_manifest" or "/MANIFEST" or "\\MANIFEST", all of length 9, from the end
		// of the path to obtain the runfiles directory.
		d := string(f)[:len(string(f))-len("_manifest")]
		env = append(env,
			directoryVar+"="+d,
			legacyDirectoryVar+"="+d)
	}
	r := &Runfiles{
		impl:       &m,
		env:        env,
		sourceRepo: string(sourceRepo),
	}
	err = r.loadRepoMapping()
	return r, err
}

type trie map[string]trie

type manifest struct {
	index map[string]string
	trie  trie
}

func (f ManifestFile) parse() (manifest, error) {
	r, err := os.Open(string(f))
	if err != nil {
		return manifest{}, fmt.Errorf("runfiles: can’t open manifest file: %w", err)
	}
	defer r.Close()

	s := bufio.NewScanner(r)
	m := manifest{make(map[string]string), nil}
	for s.Scan() {
		line := s.Text()
		var link, target string
		if strings.HasPrefix(line, " ") {
			// In lines that start with a space, spaces, newlines, and backslashes are escaped as \s, \n, and \b in
			// link and newlines and backslashes are escaped in target.
			fields := strings.SplitN(s.Text()[1:], " ", 2)
			link = fields[0]
			target = fields[1]
			link = strings.ReplaceAll(link, `\s`, " ")
			link = strings.ReplaceAll(link, `\n`, "\n")
			link = strings.ReplaceAll(link, `\b`, `\`)
			target = strings.ReplaceAll(target, `\n`, "\n")
			target = strings.ReplaceAll(target, `\b`, `\`)
		} else {
			fields := strings.SplitN(line, " ", 2)
			link = fields[0]
			target = fields[1]
		}
		m.index[link] = filepath.FromSlash(target)
	}

	if err := s.Err(); err != nil {
		return manifest{}, fmt.Errorf("runfiles: error parsing manifest file %s: %w", f, err)
	}

	return m, nil
}

func (m *manifest) path(s string) (string, error) {
	r, ok := m.index[s]
	if ok && r == "" {
		return "", ErrEmpty
	}
	if ok {
		return r, nil
	}

	// If path references a runfile that lies under a directory that itself is a
	// runfile, then only the directory is listed in the manifest. Look up all
	// prefixes of path in the manifest.
	for prefix := s; prefix != ""; prefix, _ = path.Split(prefix) {
		prefix = strings.TrimSuffix(prefix, "/")
		if prefixMatch, ok := m.index[prefix]; ok {
			return prefixMatch + filepath.FromSlash(strings.TrimPrefix(s, prefix)), nil
		}
	}

	return "", os.ErrNotExist
}

func (m *manifest) open(name string) (fs.File, error) {
	if name != "." {
		r, err := m.path(name)
		if err == ErrEmpty {
			return emptyFile(name), nil
		} else if err == nil {
			// name refers to an actual file or dir listed in the manifest. The
			// basename of name may not match the basename of the underlying
			// file (e.g. in the case of a root symlink), so patch it.
			var f *os.File
			f, err = os.Open(r)
			if err != nil {
				return nil, err
			}
			return renamedFile{f, path.Base(name)}, nil
		} else if err != os.ErrNotExist {
			return nil, err
		}
		// err == os.ErrNotExist, but name may still refer to a directory that
		// is a prefix of some manifest entry. We fall back to a trie lookup.
	}

	// At this point the file is not directly listed in the manifest (or
	// contained in a directory that is). We lazily build a trie to allow
	// efficient listing of the contents of intermediate directories.
	if m.trie == nil {
		m.trie = make(map[string]trie)
		for k := range m.index {
			segments := strings.Split(k, "/")
			current := m.trie
			for _, s := range segments {
				if current[s] == nil {
					current[s] = make(map[string]trie)
				}
				current = current[s]
			}
		}
	}

	dir := m.trie
	if name != "." {
		segments := strings.Split(name, "/")
		for _, s := range segments {
			if dir == nil {
				break
			}
			dir = dir[s]
		}
		if dir == nil {
			return nil, os.ErrNotExist
		}
	}

	entries := make([]manifestDirEntry, 0, len(dir))
	for e := range dir {
		entries = append(entries, manifestDirEntry{e, m.index[path.Join(name, e)]})
	}
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].name < entries[j].name
	})
	return &manifestReadDirFile{dirFile(path.Base(name)), entries}, nil
}

type manifestDirEntry struct {
	name string
	path string
}

type manifestReadDirFile struct {
	dirFile
	entries []manifestDirEntry
}

func (m *manifestReadDirFile) ReadDir(n int) ([]fs.DirEntry, error) {
	if n > 0 && len(m.entries) == 0 {
		return nil, io.EOF
	}
	if n <= 0 || n > len(m.entries) {
		n = len(m.entries)
	}
	entries := m.entries[:n]
	m.entries = m.entries[n:]

	dirEntries := make([]fs.DirEntry, 0, len(entries))
	for _, e := range entries {
		var info fs.FileInfo
		if e.path == "" {
			// The entry corresponds to a directory that is a prefix of some
			// manifest entry. We represent it as a read-only directory.
			info = dirFileInfo(e.name)
		} else {
			// The entry corresponds to a real file in the manifest. The
			// basename of the entry may differ from the basename of the path
			// listed as its target, so we override it.
			realInfo, err := os.Stat(e.path)
			if err != nil {
				return nil, err
			}
			info = renamedFileInfo{realInfo, e.name}
		}
		dirEntries = append(dirEntries, fs.FileInfoToDirEntry(info))
	}
	return dirEntries, nil
}

type dirFile string

func (r dirFile) Stat() (fs.FileInfo, error) { return dirFileInfo(r), nil }
func (r dirFile) Read(_ []byte) (int, error) { return 0, syscall.EISDIR }
func (r dirFile) Close() error               { return nil }

type dirFileInfo string

func (i dirFileInfo) Name() string     { return string(i) }
func (dirFileInfo) Size() int64        { return 0 }
func (dirFileInfo) Mode() fs.FileMode  { return fs.ModeDir | 0555 }
func (dirFileInfo) ModTime() time.Time { return time.Time{} }
func (dirFileInfo) IsDir() bool        { return true }
func (dirFileInfo) Sys() interface{}   { return nil }
func (i dirFileInfo) String() string   { return fs.FormatFileInfo(i) }
