package main

import (
	"fmt"
	"os"
)

func nogoValidation(args []string) error {
	if len(args) != 3 {
		return fmt.Errorf("usage: nogovalidation <validation_output> <log_file> <fix_file>\n\tgot: %v+", args)
	}
	validationOutput := args[0]
	logFile := args[1]
	fixFile := args[2]
	// Always create the output file and only fail if the log file is non-empty to
	// avoid an "action failed to create outputs" error.
	logContent, err := os.ReadFile(logFile)
	if err != nil {
		return err
	}
	err = os.WriteFile(validationOutput, logContent, 0755)
	if err != nil {
		return err
	}
	if len(logContent) > 0 {
		fixContent, err := os.ReadFile(fixFile)
		if err != nil {
			return err
		}
		var fixMessage string
		if len(fixContent) > 0 {
			// Format the message in a clean and clear way
			fixMessage = fmt.Sprintf(`
-------------------Suggested Fix---------------------
%s
-----------------------------------------------------
To apply the suggested fix, run the following command:
$ patch -p1 < %s
`, fixContent, fixFile)
		}
		// Separate nogo output from Bazel's --sandbox_debug message via an
		// empty line.
		// Don't return to avoid printing the "nogovalidation:" prefix.
		_, _ = fmt.Fprintf(os.Stderr, "\n%s%s\n", logContent, fixMessage)
		os.Exit(1)
	}
	return nil
}
