package main

import (
	"bufio"
	"fmt"
	"go/build/constraint"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

var goVersionRegex = regexp.MustCompile(`^go1.(\d+)$`)

// Used to update the list of tags affecting the standard library kept in
// transitions.bzl.
func main() {
	if len(os.Args) < 2 {
		log.Fatal("usage: stdlib_tags <go SDK src directory>...")
	}

	filteredTags, err := extractBuildTags(os.Args[1:]...)
	if err != nil {
		log.Fatal(err.Error())
	}

	fmt.Printf("_TAG_AFFECTS_STDLIB = {\n")
	for _, tag := range filteredTags {
		fmt.Printf("    %q: None,\n", tag)
	}
	fmt.Printf("}\n")
}

func extractBuildTags(sdkPaths ...string) ([]string, error) {
	tags := make(map[string]struct{})
	for _, dir := range sdkPaths {
		err := filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
			if d.IsDir() {
				if d.Name() == "testdata" {
					return filepath.SkipDir
				}
				return nil
			}
			if filepath.Ext(path) != ".go" {
				return nil
			}
			if strings.HasSuffix(filepath.Base(path), "_test.go") {
				return nil
			}
			return walkFile(path, tags)
		})
		if err != nil {
			return nil, fmt.Errorf("%s: %w", dir, err)
		}
	}

	filteredTags := make([]string, 0, len(tags))
	for tag := range tags {
		if !shouldExclude(tag) {
			filteredTags = append(filteredTags, tag)
		}
	}
	sort.Strings(filteredTags)

	return filteredTags, nil
}

func shouldExclude(tag string) bool {
	// Set via CGO_ENABLED
	return tag == "cgo" ||
		// Set via GOARCH and GOOS
		knownOS[tag] || knownArch[tag] || tag == "unix" ||
		// Set via GOEXPERIMENT and GOAMD64
		strings.HasPrefix(tag, "goexperiment.") || strings.HasPrefix(tag, "amd64.") ||
		// Set implicitly
		goVersionRegex.MatchString(tag)
}

func walkFile(path string, tags map[string]struct{}) error {
	file, err := os.Open(path)
	if err != nil {
		return err
	}

	scanner := bufio.NewScanner(file)
	// The Go SDK contains some very long lines in vendored files (minified JS).
	scanner.Buffer(make([]byte, 0, 128*1024), 2*1024*1024)
	for scanner.Scan() {
		line := scanner.Text()
		if !isConstraint(line) {
			continue
		}
		c, err := constraint.Parse(line)
		if err != nil {
			continue
		}
		walkConstraint(c, tags)
	}

	if err = scanner.Err(); err != nil {
		return fmt.Errorf("%s: %w", path, err)
	}

	return nil
}

func walkConstraint(c constraint.Expr, tags map[string]struct{}) {
	switch c.(type) {
	case *constraint.AndExpr:
		walkConstraint(c.(*constraint.AndExpr).X, tags)
		walkConstraint(c.(*constraint.AndExpr).Y, tags)
	case *constraint.OrExpr:
		walkConstraint(c.(*constraint.OrExpr).X, tags)
		walkConstraint(c.(*constraint.OrExpr).Y, tags)
	case *constraint.NotExpr:
		walkConstraint(c.(*constraint.NotExpr).X, tags)
	case *constraint.TagExpr:
		tags[c.(*constraint.TagExpr).Tag] = struct{}{}
	}
}

func isConstraint(line string) bool {
	return constraint.IsPlusBuild(line) || constraint.IsGoBuild(line)
}

// Taken from
// https://github.com/golang/go/blob/2693f77b3583585172810427e12a634b28d34493/src/internal/syslist/syslist.go
var knownOS = map[string]bool{
	"aix":       true,
	"android":   true,
	"darwin":    true,
	"dragonfly": true,
	"freebsd":   true,
	"hurd":      true,
	"illumos":   true,
	"ios":       true,
	"js":        true,
	"linux":     true,
	"nacl":      true,
	"netbsd":    true,
	"openbsd":   true,
	"plan9":     true,
	"solaris":   true,
	"wasip1":    true,
	"windows":   true,
	"zos":       true,
}

var knownArch = map[string]bool{
	"386":         true,
	"amd64":       true,
	"amd64p32":    true,
	"arm":         true,
	"armbe":       true,
	"arm64":       true,
	"arm64be":     true,
	"loong64":     true,
	"mips":        true,
	"mipsle":      true,
	"mips64":      true,
	"mips64le":    true,
	"mips64p32":   true,
	"mips64p32le": true,
	"ppc":         true,
	"ppc64":       true,
	"ppc64le":     true,
	"riscv":       true,
	"riscv64":     true,
	"s390":        true,
	"s390x":       true,
	"sparc":       true,
	"sparc64":     true,
	"wasm":        true,
}
