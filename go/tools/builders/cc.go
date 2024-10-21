package main

import (
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
)

func cc(args []string) error {
	cc := os.Getenv("GO_CC")
	if cc == "" {
		log.Fatal("GO_CC environment variable not set")
	}
	ccroot := os.Getenv("GO_CC_ROOT")
	if ccroot == "" {
		log.Fatal("GO_CC_ROOT environment variable not set")
	}

	normalized := []string{cc}
	normalized = append(normalized, args...)
	transformArgs(normalized, cgoAbsEnvFlags, func(s string) string {
		if strings.HasPrefix(s, cgoAbsPlaceholder) {
			trimmed := strings.TrimPrefix(s, cgoAbsPlaceholder)
			abspath := filepath.Join(ccroot, trimmed)
			if _, err := os.Stat(abspath); err == nil {
				// Only return the abspath if it exists, otherwise it
				// means that either it won't have any effect or the original
				// value was not a relpath (e.g. a path with a XCODE placehold from
				// macos cc_wrapper)
				return abspath
			}
			return trimmed
		}
		return s
	})
	if runtime.GOOS == "windows" {
		cmd := exec.Command(normalized[0], normalized[1:]...)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		return cmd.Run()
	} else {
		return syscall.Exec(normalized[0], normalized, os.Environ())
	}
}
