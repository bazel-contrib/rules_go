package main

import (
	"path/filepath"
	"testing"
)

func TestImportGraphHasCgoFound(t *testing.T) {
	dir := t.TempDir()

	// main → fmt, mylib
	// mylib → net
	// net → runtime/cgo
	mainA := buildTestArchive(t, dir, "main.a", []string{"fmt", "mylib"})
	mylibA := buildTestArchive(t, dir, "mylib.a", []string{"net"})
	netA := buildTestArchive(t, dir, "net.a", []string{"runtime/cgo"})
	buildTestArchive(t, dir, "fmt.a", []string{"io"})
	buildTestArchive(t, dir, "io.a", nil)

	pkgToFile := map[string]string{
		"fmt":   filepath.Join(dir, "fmt.a"),
		"mylib": mylibA,
		"net":   netA,
		"io":    filepath.Join(dir, "io.a"),
	}

	got, err := importGraphHasCgo(mainA, pkgToFile)
	if err != nil {
		t.Fatal(err)
	}
	if !got {
		t.Error("importGraphHasCgo = false, want true (net imports runtime/cgo)")
	}
}

func TestImportGraphHasCgoNotFound(t *testing.T) {
	dir := t.TempDir()

	// main → fmt → io (no runtime/cgo anywhere)
	mainA := buildTestArchive(t, dir, "main.a", []string{"fmt"})
	buildTestArchive(t, dir, "fmt.a", []string{"io"})
	buildTestArchive(t, dir, "io.a", nil)

	pkgToFile := map[string]string{
		"fmt": filepath.Join(dir, "fmt.a"),
		"io":  filepath.Join(dir, "io.a"),
	}

	got, err := importGraphHasCgo(mainA, pkgToFile)
	if err != nil {
		t.Fatal(err)
	}
	if got {
		t.Error("importGraphHasCgo = true, want false (no runtime/cgo in graph)")
	}
}

func TestImportGraphHasCgoDirectImport(t *testing.T) {
	dir := t.TempDir()

	// main directly imports runtime/cgo
	mainA := buildTestArchive(t, dir, "main.a", []string{"runtime/cgo", "fmt"})
	buildTestArchive(t, dir, "fmt.a", nil)

	pkgToFile := map[string]string{
		"fmt": filepath.Join(dir, "fmt.a"),
	}

	got, err := importGraphHasCgo(mainA, pkgToFile)
	if err != nil {
		t.Fatal(err)
	}
	if !got {
		t.Error("importGraphHasCgo = false, want true (main directly imports runtime/cgo)")
	}
}

func TestImportGraphHasCgoCycle(t *testing.T) {
	dir := t.TempDir()

	// main → a → b → a (cycle, no cgo)
	mainA := buildTestArchive(t, dir, "main.a", []string{"a"})
	buildTestArchive(t, dir, "a.a", []string{"b"})
	buildTestArchive(t, dir, "b.a", []string{"a"})

	pkgToFile := map[string]string{
		"a": filepath.Join(dir, "a.a"),
		"b": filepath.Join(dir, "b.a"),
	}

	got, err := importGraphHasCgo(mainA, pkgToFile)
	if err != nil {
		t.Fatal(err)
	}
	if got {
		t.Error("importGraphHasCgo = true, want false (cycle but no runtime/cgo)")
	}
}

func TestImportGraphHasCgoUnresolved(t *testing.T) {
	dir := t.TempDir()

	// main → unknown_pkg (not in pkgToFile)
	mainA := buildTestArchive(t, dir, "main.a", []string{"unknown_pkg"})

	pkgToFile := map[string]string{}

	got, err := importGraphHasCgo(mainA, pkgToFile)
	if err != nil {
		t.Fatal(err)
	}
	if got {
		t.Error("importGraphHasCgo = true, want false (unresolved deps should be skipped)")
	}
}
