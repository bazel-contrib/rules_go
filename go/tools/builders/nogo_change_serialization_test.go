package main

import (
	"os"
	"testing"
)

// TestSaveAndLoadPatches tests both SavePatchesToFile and LoadPatchesFromFile functions.
func TestSaveAndLoadPatches(t *testing.T) {
	// Create a temporary file for testing
	tempFile, err := os.CreateTemp("", "patches_test_*.json")
	if err != nil {
		t.Fatalf("Failed to create temporary file: %v", err)
	}
	defer os.Remove(tempFile.Name()) // Clean up the temp file after the test

	// Define the test data (map[string]string)
	patches := map[string]string{
		"file1.go": "patch content for file1",
		"file2.go": "patch content for file2",
	}

	// Test SavePatchesToFile
	err = SavePatchesToFile(tempFile.Name(), patches)
	if err != nil {
		t.Fatalf("SavePatchesToFile failed: %v", err)
	}

	// Test LoadPatchesFromFile
	loadedPatches, err := LoadPatchesFromFile(tempFile.Name())
	if err != nil {
		t.Fatalf("LoadPatchesFromFile failed: %v", err)
	}

	// Check if the loaded patches match the original ones
	if len(loadedPatches) != len(patches) {
		t.Errorf("Expected %d patches, but got %d", len(patches), len(loadedPatches))
	}

	for key, value := range patches {
		if loadedPatches[key] != value {
			t.Errorf("Patch mismatch for key %s: expected %s, got %s", key, value, loadedPatches[key])
		}
	}

	// Test with an empty map
	patches = map[string]string{}
	err = SavePatchesToFile(tempFile.Name(), patches)
	if err != nil {
		t.Fatalf("SavePatchesToFile failed for empty map: %v", err)
	}

	loadedPatches, err = LoadPatchesFromFile(tempFile.Name())
	if err != nil {
		t.Fatalf("LoadPatchesFromFile failed for empty map: %v", err)
	}

	// Check if the loaded patches map is empty
	if len(loadedPatches) != 0 {
		t.Errorf("Expected empty patches map, but got %d entries", len(loadedPatches))
	}
}

// TestSavePatchesToFileError tests error handling in SavePatchesToFile.
func TestSavePatchesToFileError(t *testing.T) {
	// Invalid file path (simulating write error)
	filename := "/invalid/path/patches.json"
	patches := map[string]string{
		"file1.go": "patch content",
	}

	err := SavePatchesToFile(filename, patches)
	if err == nil {
		t.Errorf("Expected error when saving to invalid path, but got nil")
	}
}

// TestLoadPatchesFromFileError tests error handling in LoadPatchesFromFile.
func TestLoadPatchesFromFileError(t *testing.T) {
	// Invalid file path (simulating read error)
	filename := "/invalid/path/patches.json"

	_, err := LoadPatchesFromFile(filename)
	if err == nil {
		t.Errorf("Expected error when loading from invalid path, but got nil")
	}

	// Invalid JSON content
	tempFile, err := os.CreateTemp("", "invalid_json_*.json")
	if err != nil {
		t.Fatalf("Failed to create temporary file: %v", err)
	}
	defer os.Remove(tempFile.Name()) // Clean up

	// Write invalid JSON content to the file
	_, err = tempFile.WriteString("invalid json content")
	if err != nil {
		t.Fatalf("Failed to write invalid content: %v", err)
	}

	// Attempt to load invalid JSON content
	_, err = LoadPatchesFromFile(tempFile.Name())
	if err == nil {
		t.Errorf("Expected error when loading invalid JSON, but got nil")
	}
}

// TestLoadPatchesFromFileEmptyFile tests the case where the file is empty.
func TestLoadPatchesFromFileEmptyFile(t *testing.T) {
	// Create a temporary file for testing (empty file)
	tempFile, err := os.CreateTemp("", "empty_file_*.json")
	if err != nil {
		t.Fatalf("Failed to create temporary file: %v", err)
	}
	defer os.Remove(tempFile.Name()) // Clean up the temp file after the test

	// Ensure the file is empty
	err = os.WriteFile(tempFile.Name(), []byte(""), 0644)
	if err != nil {
		t.Fatalf("Failed to write empty content to file: %v", err)
	}

	// Attempt to load from an empty file
	loadedPatches, err := LoadPatchesFromFile(tempFile.Name())
	if err != nil {
		t.Fatalf("LoadPatchesFromFile failed for empty file: %v", err)
	}

	// Check if the loaded patches map is empty
	if len(loadedPatches) != 0 {
		t.Errorf("Expected empty patches map from empty file, but got %d entries", len(loadedPatches))
	}
}
