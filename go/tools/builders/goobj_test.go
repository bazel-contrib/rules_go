package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestReadAutolibImportsBasic(t *testing.T) {
	dir := t.TempDir()
	path := buildTestArchive(t, dir, "test.a", []string{"fmt", "runtime", "runtime/cgo"})

	imports, err := readAutolibImports(path)
	if err != nil {
		t.Fatalf("readAutolibImports: %v", err)
	}

	want := map[string]bool{"fmt": true, "runtime": true, "runtime/cgo": true}
	got := make(map[string]bool)
	for _, imp := range imports {
		got[imp] = true
	}
	if len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	for k := range want {
		if !got[k] {
			t.Errorf("missing import %q", k)
		}
	}
}

func TestReadAutolibImportsEmpty(t *testing.T) {
	dir := t.TempDir()
	path := buildTestArchive(t, dir, "empty.a", nil)

	imports, err := readAutolibImports(path)
	if err != nil {
		t.Fatalf("readAutolibImports: %v", err)
	}
	if len(imports) != 0 {
		t.Errorf("expected no imports, got %v", imports)
	}
}

func TestReadAutolibImportsNotAnArchive(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "bad.a")
	os.WriteFile(path, []byte("not an archive"), 0644)

	_, err := readAutolibImports(path)
	if err == nil {
		t.Fatal("expected error for non-archive file")
	}
}

func TestReadAutolibImportsMissing(t *testing.T) {
	_, err := readAutolibImports("/nonexistent/path.a")
	if err == nil {
		t.Fatal("expected error for missing file")
	}
}
