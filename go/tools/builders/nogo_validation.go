package main

import (
	"fmt"
	"os"
	"path/filepath"
)

func nogoValidation(args []string) error {
	if len(args) != 2 {
		return fmt.Errorf("usage: nogovalidation <validation_output> <out_dir>\n\tgot: %v+", args)
	}
	// Always create the output file to avoid an unhelpful "action failed to create outputs" error.
	out, err := os.Create(args[0])
	if err != nil {
		return err
	}
	defer out.Close()

	logFile := filepath.Join(args[1], nogoLogBasename)
	logContent, err := os.ReadFile(logFile)
	if os.IsNotExist(err) {
		// No nogo findings, this is the only case in which we exit without
		// an error.
		return nil
	} else if err != nil {
		return fmt.Errorf("error reading nogo log file: %w", err)
	}

	var fixMessage string
	fixFile := filepath.Join(args[1], nogoFixBasename)
	fixContent, err := os.ReadFile(fixFile)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
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
	// Not reached
	return nil
}
