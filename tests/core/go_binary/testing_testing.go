package main

import (
	"fmt"
	"testing"
	"os"
)

func main() {
	if testing.Testing() {
		panic("testing.Testing() returned 'true' in a binary")
	}

	file, err := os.Create(os.Args[1])
	if err != nil {
		panic(fmt.Sprintf("Failed to open output file %s", err))
	}
	file.Close()
}
