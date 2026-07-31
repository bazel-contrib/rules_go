//go:build !windows

package main

func processPath(path string) (string, error) {
	return path, nil
}
