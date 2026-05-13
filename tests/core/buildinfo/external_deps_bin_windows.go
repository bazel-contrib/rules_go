//go:build windows

package main

import (
	"fmt"
	"runtime/debug"

	"golang.org/x/sys/windows"
)

func main() {
	// Use the external dependency
	_ = windows.ERROR_FILE_NOT_FOUND

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
