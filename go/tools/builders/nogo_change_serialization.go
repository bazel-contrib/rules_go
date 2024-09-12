package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	// "log"
)

// SaveToFile saves the Change struct to a JSON file.
func SaveToFile(filename string, change Change) error {
	// Serialize Change to JSON
	jsonData, err := json.MarshalIndent(change, "", "  ")
	if err != nil {
		return fmt.Errorf("error serializing to JSON: %v", err)
	}
	// log.Fatalf("!!!!: %v", change)
	// Write the JSON data to the file
	err = ioutil.WriteFile(filename, jsonData, 0644)
	if err != nil {
		return fmt.Errorf("error writing to file: %v", err)
	}

	return nil
}

// LoadFromFile loads the Change struct from a JSON file.
func LoadFromFile(filename string) (Change, error) {
	var change Change

	// Read the JSON file
	jsonData, err := ioutil.ReadFile(filename)
	if err != nil {
		return change, fmt.Errorf("error reading file: %v", err)
	}

	// Deserialize JSON data into the Change struct
	err = json.Unmarshal(jsonData, &change)
	if err != nil {
		return change, fmt.Errorf("error deserializing JSON: %v", err)
	}

	return change, nil
}
