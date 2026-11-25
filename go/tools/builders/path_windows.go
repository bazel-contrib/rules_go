//go:build windows

package main

import (
	"runtime"
	"syscall"
)

func processPath(path string) (string, error) {
	if runtime.GOOS != "windows" {
		return path, nil
	}

	var buf [258]uint16
	up, err := syscall.UTF16PtrFromString(path)
	if err != nil {
		return path, err
	}
	_, err = syscall.GetShortPathName(up, &buf[0], 258)
	if err != nil {
		return path, err
	}
	return syscall.UTF16ToString(buf[:]), nil
}
