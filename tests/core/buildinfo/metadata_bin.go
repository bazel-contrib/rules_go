package main

import (
	"fmt"
	"runtime/debug"

	"github.com/bazelbuild/rules_go/tests/core/buildinfo/top"
)

func main() {
	fmt.Println(top.TopFunc())

	// Print buildInfo for testing
	info, ok := debug.ReadBuildInfo()
	if !ok {
		fmt.Println("NO_BUILD_INFO")
		return
	}

	fmt.Printf("Path=%s\n", info.Path)
	fmt.Printf("GoVersion=%s\n", info.GoVersion)

	// Print dependencies in sorted order
	for _, dep := range info.Deps {
		fmt.Printf("Dep=%s@%s\n", dep.Path, dep.Version)
	}
}
