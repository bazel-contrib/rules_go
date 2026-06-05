//go:build unix

package main

import (
	"fmt"
	"runtime/debug"

	"golang.org/x/sys/unix"
)

func main() {
	// Use the external dependency
	_ = unix.ENOENT

	// Print buildInfo for testing
	info, ok := debug.ReadBuildInfo()
	if !ok {
		fmt.Println("NO_BUILD_INFO")
		return
	}

	fmt.Printf("Path=%s\n", info.Path)

	// Print dependencies in sorted order
	for _, dep := range info.Deps {
		fmt.Printf("Dep=%s@%s\n", dep.Path, dep.Version)
	}
}
