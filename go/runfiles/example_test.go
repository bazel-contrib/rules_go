package runfiles_test

import (
	"io/fs"
	"os"
	"os/exec"

	"github.com/bazelbuild/rules_go/go/runfiles"
)

// Read a runfile.
func Example() {
	p, err := runfiles.Rlocation("my_module/path/to/pkg/file.txt")
	if err != nil {
		panic(err)
	}
	content, _ := os.ReadFile(p)
	println(string(content))
}

// Read a runfile.
func ExampleRlocation() {
	p, err := runfiles.Rlocation("my_module/path/to/pkg/file.txt")
	if err != nil {
		panic(err)
	}
	content, _ := os.ReadFile(p)
	println(string(content))
}

// Execute a tool from runfiles that itself uses runfiles.
func ExampleEnv() {
	tool, err := runfiles.Rlocation("my_module/path/to/pkg/some_tool")
	if err != nil {
		panic(err)
	}
	runfilesEnv, err := runfiles.Env()
	if err != nil {
		panic(err)
	}
	cmd := exec.Command(tool, "arg1", "arg2")
	cmd.Env = append(os.Environ(), runfilesEnv...)
	_ = cmd.Run()
}

// Copy a subdirectory of the runfiles to a temporary directory.
func ExampleNew_copy() {
	r, err := runfiles.New()
	if err != nil {
		panic(err)
	}
	tmpDir, _ := os.MkdirTemp("", "example")
	defer os.RemoveAll(tmpDir)
	testdataFS, _ := fs.Sub(r, "my_module/path/to/pkg/testdata")
	_ = os.CopyFS(tmpDir, testdataFS)
}

// Iterate over all ".txt" files in a runfiles subdirectory.
func ExampleNew_glob() {
	r, err := runfiles.New()
	if err != nil {
		panic(err)
	}
	textFiles, _ := fs.Glob(r, "my_module/path/to/pkg/testdata/*.txt")
	for _, f := range textFiles {
		content, _ := fs.ReadFile(r, f)
		println(string(content))
	}
}
