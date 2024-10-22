package main

import (
	"fmt"
	"os"
)

func nogoValidation(args []string) error {
	validationOutput := args[0]
	logFile := args[1]
	nogoFixFileTmp := args[2]
	nogoFixFile := args[3]

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

	nogoFixContent, err := os.ReadFile(nogoFixFileTmp)
	if err != nil {
		return err
	}
	err = os.WriteFile(nogoFixFile, nogoFixContent, 0755)
	if err != nil {
		return err
	}

	if len(logContent) > 0 {
		nogoFixRelated := ""
		// See nogo_change_serialization.go, if the patches are empty, then nogoFixContent is empty by design, rather than an empty json like {}.
		if len(nogoFixContent) > 0 {
			// Command to view nogo fix
			viewNogoFixCmd := fmt.Sprintf("jq -r 'to_entries[] | .value | @text' %s | tee", nogoFixFile)
			// Command to apply nogo fix
			applyNogoFixCmd := fmt.Sprintf("jq -r 'to_entries[] | .value | @text' %s | patch -p1", nogoFixFile)

			// Format the message in a clean and clear way
			nogoFixRelated = fmt.Sprintf(`
--------------------------------------
To view the nogo fix, run the following command:
$ %s

To apply the nogo fix, run the following command:
$ %s
--------------------------------------
		`, viewNogoFixCmd, applyNogoFixCmd)
		}
		// Separate nogo output from Bazel's --sandbox_debug message via an
		// empty line.
		// Don't return to avoid printing the "nogovalidation:" prefix.
		_, _ = fmt.Fprintf(os.Stderr, "\n%s%s\n", logContent, nogoFixRelated)
		os.Exit(1)
	}
	return nil
}
