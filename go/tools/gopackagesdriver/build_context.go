package main

import (
	"go/build"
	"path/filepath"
	"strings"
)

var buildContext = makeBuildContext()

func makeBuildContext() *build.Context {
	bctx := build.Default
	bctx.BuildTags = strings.Split(getenvDefault("GOTAGS", ""), ",")

	return &bctx
}

func filterSourceFilesForTags(files []string) []string {
	ret := make([]string, 0, len(files))

	for _, f := range files {
		dir, filename := filepath.Split(f)
		ext := filepath.Ext(f)

		match, _ := buildContext.MatchFile(dir, filename)
		// MatchFile filters out anything without a file extension. In the
		// case of CompiledGoFiles (in particular cgo processed files from
		// the cache), we want them. We also want to keep cgo processed files
		// with known naming conventions.
		if match || ext == "" || isCgoProcessed(filename) {
			ret = append(ret, f)
		}
	}
	return ret
}

// isCgoProcessed returns true if the file is a cgo processed file.
func isCgoProcessed(fileName string) bool {
	// go/tools/builders/cgo2.go generates cgo processed code that are
	// named *.cgo1.go, _cgo_gotypes.go, and _cgo_imports.go.
	return fileName == "_cgo_gotypes.go" || fileName == "_cgo_imports.go" || strings.HasSuffix(fileName, ".cgo1.go")
}
