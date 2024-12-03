package runfiles_test

import (
	"os"
	"path"

	"github.com/bazelbuild/rules_go/go/runfiles"
)

// MyRepoRelativeRlocation behaves like runfiles.Rlocation, but accepts a path
// relative to the repository of the caller rather than a path prefixed with a
// repository name.
func MyRepoRelativeRlocation(relativePath string) (string, error) {
	var runfilesDirectory string
	callerRepository := runfiles.CallerRepository()
	if callerRepository == "" {
		// The runfiles directory of the main repository has a special name
		// with Bzlmod.
		runfilesDirectory = "_main"
	} else {
		runfilesDirectory = callerRepository
	}
	rlocationPath := path.Join(runfilesDirectory, relativePath)
	// runfiles.CallerRepository always returns a canonical repository name,
	// but we still specify the correct source repo here in case the caller
	// specifies a path such as "../other_repo/file.txt".
	return runfiles.RlocationFrom(rlocationPath, callerRepository)
}

// Implement a custom wrapper around runfiles.Rlocation that accepts paths
// relative to the repository of the caller.
func ExampleCallerRepository() {
	p, err := MyRepoRelativeRlocation("path/to/pkg/file.txt")
	if err != nil {
		panic(err)
	}
	content, _ := os.ReadFile(p)
	println(string(content))
}
