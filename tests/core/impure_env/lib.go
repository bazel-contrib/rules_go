package impure_env

import "os"

// GetTestVar returns the value of TEST_VAR environment variable
func GetTestVar() string {
	return os.Getenv("TEST_VAR")
}

// GetMultipleVars returns a map of multiple environment variables
func GetMultipleVars() map[string]string {
	return map[string]string{
		"TEST_VAR1": os.Getenv("TEST_VAR1"),
		"TEST_VAR2": os.Getenv("TEST_VAR2"),
		"TEST_VAR3": os.Getenv("TEST_VAR3"),
	}
}
