package main

import (
	"fmt"
	"os"
)

func nogoValidation(args []string) error {
	validationOutput := args[0]
	logFile := args[1]
	nogoFixFile := args[2]

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

	nogoFixContent, err := os.ReadFile(nogoFixFile)
	if err != nil {
		return err
	}

	if len(logContent) > 0 {
		nogoFixRelated := ""
		// See nogo_change_serialization.go, if the patches are empty, then nogoFixContent is empty by design, rather than an empty json like {}.
		if len(nogoFixContent) > 0 {
			// Format the message in a clean and clear way
			nogoFixRelated = fmt.Sprintf(`
-------------------Suggested Fixes-------------------
The suggested fixes are as follows:
%s

To apply the suggested fixes, run the following command:
$ patch -p1 < %s
-----------------------------------------------------
`, nogoFixContent, nogoFixFile)
		}
		// Separate nogo output from Bazel's --sandbox_debug message via an
		// empty line.
		// Don't return to avoid printing the "nogovalidation:" prefix.
		_, _ = fmt.Fprintf(os.Stderr, "\n%s%s\n", logContent, nogoFixRelated)
		os.Exit(1)
	}
	return nil
}
