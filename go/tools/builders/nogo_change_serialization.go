package main

import (
	"encoding/json"
	"fmt"
	"os"
)

// SavePatchesToFile saves the map[string]string (file paths to patch content) to a JSON file.
func SavePatchesToFile(filename string, patches map[string]string) error {
	if len(patches) == 0 {
		// Special case optimization for the empty patches, where we dump an empty string, rather than an empty json like {}.
		// This helps skip the json serialization below.
		err := os.WriteFile(filename, []byte(""), 0644)
		if err != nil {
			return fmt.Errorf("error writing empty string to file: %v", err)
		}
		return nil
	}

	// Serialize patches (map[string]string) to JSON
	jsonData, err := json.MarshalIndent(patches, "", "  ")
	if err != nil {
		// If serialization fails, create the output file anyway as per your requirements
		errWrite := os.WriteFile(filename, []byte(""), 0644)
		if errWrite != nil {
			return fmt.Errorf("error serializing to JSON: %v and error writing to the file: %v", err, errWrite)
		} else {
			return fmt.Errorf("error serializing to JSON: %v", err)
		}
	}

	// Write the JSON data to the file
	err = os.WriteFile(filename, jsonData, 0644)
	if err != nil {
		return fmt.Errorf("error writing to file: %v", err)
	}

	return nil
}

// LoadPatchesFromFile loads the map[string]string (file paths to patch content) from a JSON file.
// Note LoadPatchesFromFile is used for testing only.
func LoadPatchesFromFile(filename string) (map[string]string, error) {
	var patches map[string]string

	// Read the JSON file
	jsonData, err := os.ReadFile(filename)
	if err != nil {
		return nil, fmt.Errorf("error reading file: %v", err)
	}

	if len(jsonData) == 0 {
		// this corresponds to the special case optimization in SavePatchesToFile
		return make(map[string]string), nil
	}

	// Deserialize JSON data into the patches map (map[string]string)
	err = json.Unmarshal(jsonData, &patches)
	if err != nil {
		return nil, fmt.Errorf("error deserializing JSON: %v", err)
	}

	return patches, nil
}
