package main

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"
)

type test struct {
	name string
	in   string
	out  string
}

var tests = []test{
	{
		name: "no imports",
		in: `package main
`,
		out: `package main; import "github.com/bazelbuild/rules_go/go/tools/coverdata"

func init() {
	coverdata.RegisterSrcPathMapping("some.importh/path/file.go", "src/path/file.go")
}
`,
	},
	{
		name: "other imports",
		in: `package main

import (
	"os"
)
`,
		out: `package main; import "github.com/bazelbuild/rules_go/go/tools/coverdata"

import (
	"os"
)

func init() {
	coverdata.RegisterSrcPathMapping("some.importh/path/file.go", "src/path/file.go")
}
`,
	},
	{
		name: "existing import",
		in: `package main

import "github.com/bazelbuild/rules_go/go/tools/coverdata"
`,
		out: `package main

import "github.com/bazelbuild/rules_go/go/tools/coverdata"

func init() {
	coverdata.RegisterSrcPathMapping("some.importh/path/file.go", "src/path/file.go")
}
`,
	},
	{
		name: "existing _ import",
		in: `package main

import _ "github.com/bazelbuild/rules_go/go/tools/coverdata"
`,
		out: `package main

import coverdata "github.com/bazelbuild/rules_go/go/tools/coverdata"

func init() {
	coverdata.RegisterSrcPathMapping("some.importh/path/file.go", "src/path/file.go")
}
`,
	},
	{
		name: "existing renamed import",
		in: `package main

import cover0 "github.com/bazelbuild/rules_go/go/tools/coverdata"
`,
		out: `package main

import cover0 "github.com/bazelbuild/rules_go/go/tools/coverdata"

func init() {
	cover0.RegisterSrcPathMapping("some.importh/path/file.go", "src/path/file.go")
}
`,
	},
}

func TestRegisterCoverage(t *testing.T) {
	var filename = filepath.Join(t.TempDir(), "test_input.go")
	for _, test := range tests {
		if err := ioutil.WriteFile(filename, []byte(test.in), 0666); err != nil {
			t.Errorf("writing input file: %v", err)
			return
		}
		err := registerCoverage(filename, "some.importh/path/file.go", "src/path/file.go")
		if err != nil {
			t.Errorf("%q: %+v", test.name, err)
			continue
		}
		coverSrc, err := os.ReadFile(filename)
		if err != nil {
			t.Errorf("%q: %+v", test.name, err)
			continue
		}
		if got, want := string(coverSrc), test.out; got != want {
			t.Errorf("%q: got %v, want %v", test.name, got, want)
		}
	}
}

func TestFindFirstLineDirectiveFilename(t *testing.T) {
	tests := []struct {
		desc string
		src string
		content string
		want string
	}{
		{desc: "no line directive", src: "noline.go", content: "package main\n\nfunc main() {}\n", want: ""},
		{desc: "slash slash directive", src: "slashslash.go", content: "package main\n\n//line foo.go:123\n\nfunc main() {}\n", want: "foo.go"},
		{desc: "slash star directive", src: "slashstar.go", content: "package main\n\n/*line foo.go:123*/", want: "foo.go"},
		{desc: "windows absolute path", src: "windows.go", content: "package main\n\n//line C:\\Windows\\path.go:300", want: "C:\\Windows\\path.go"},
		{desc: "windows with column", src: "windows.go", content: "package main\n\n//line C:\\Windows\\path.go:300:10", want: "C:\\Windows\\path.go"},
		{desc: "url-like path", src: "url.go", content: "package main\n\n//line http://example.com/file.go:400", want: "http://example.com/file.go"},
		{desc: "multiple colons", src: "multiple.go", content: "package main\n\n//line scheme:rest:file.go:500", want: "scheme:rest:file.go"},
		{desc: "empty filename", src: "empty.go", content: "package main\n\n//line :50", want: ""},
		{desc: "invalid line number", src: "invalid.go", content: "package main\n\n//line file.go:0", want: ""},
		{desc: "invalid line number", src: "invalid.go", content: "package main\n\n//line file.go:invalid", want: ""},
		{desc: "not a directive", src: "notadirective.go", content: "package main\n\nnot a line directive", want: ""},
		{desc: "incomplete", src: "incomplete.go", content: "package main\n\n//line", want: ""},
		{desc: "missing line number", src: "missing.go", content: "package main\n\n//line file.go:", want: ""},
	}

	for _, test := range tests {
		testTempDir := t.TempDir()
		testFile := filepath.Join(testTempDir, test.src)
		err := os.WriteFile(testFile, []byte(test.content), 0644)
		if err != nil {
			t.Errorf("create test file: %v", err)
		}
		defer os.Remove(testFile) // Clean up the test file.

		got, err := findFirstLineDirectiveFilename(testFile)
		if err != nil {
			t.Errorf("find first line directive filename: %v", err)
		}
		if got != test.want {
			t.Errorf("find first line directive filename: got %q, want %q", got, test.want)
		}
	}
}
