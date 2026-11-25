// Copyright 2021 The Bazel Authors. All rights reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//    http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// releaser is a tool for maintaining rules_go. It automates the process of
// upgrading dependencies declared in go_rules_dependencies. Releases
// themselves are performed by the GitHub Actions workflows in
// .github/workflows, which run when a release tag is pushed.
package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/signal"
)

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()
	if err := run(ctx, os.Stderr, os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

type command struct {
	name, description, help string
	run                     func(context.Context, io.Writer, []string) error
}

var commands = []*command{
	&helpCmd,
	&upgradeDepCmd,
}

func run(ctx context.Context, stderr io.Writer, args []string) error {
	if len(args) == 0 {
		return errors.New("no command specified. For a list of commands, run:\n\treleaser help")
	}
	name, args := args[0], args[1:]
	for _, arg := range args {
		if arg == "-h" || name == "-help" || name == "--help" {
			return helpCmd.run(ctx, stderr, args)
		}
	}
	for _, cmd := range commands {
		if cmd.name == name {
			return cmd.run(ctx, stderr, args)
		}
	}
	return fmt.Errorf("unknown command %q. For a list of commands, run:\n\treleaser help", name)
}

var helpCmd = command{
	name:        "help",
	description: "prints information on how to use any subcommand",
	help: `releaser help [subcommand]

The help sub-command prints information on how to use any subcommand. Run help
without arguments for a list of all subcommands.
`,
}

func init() {
	// break init cycle
	helpCmd.run = runHelp
}

func runHelp(ctx context.Context, stderr io.Writer, args []string) error {
	if len(args) > 1 {
		return usageErrorf(&helpCmd, "help accepts at most one argument.")
	}

	if len(args) == 1 {
		name := args[0]
		for _, cmd := range commands {
			if cmd.name == name {
				fmt.Fprintf(stderr, "%s\n\n%s\n", cmd.description, cmd.help)
				return nil
			}
		}
		return fmt.Errorf("Unknown command %s. For a list of supported subcommands, run:\n\treleaser help", name)
	}

	fmt.Fprint(stderr, "releaser supports the following subcommands:\n\n")
	maxNameLen := 0
	for _, cmd := range commands {
		if len(cmd.name) > maxNameLen {
			maxNameLen = len(cmd.name)
		}
	}
	for _, cmd := range commands {
		fmt.Fprintf(stderr, "\t%-*s    %s\n", maxNameLen, cmd.name, cmd.description)
	}
	fmt.Fprintf(stderr, "\nRun 'releaser help <subcommand>' for more information on any command.\n")
	return nil
}

type usageError struct {
	cmd *command
	err error
}

func (e *usageError) Error() string {
	return fmt.Sprintf("%v. For usage info, run:\n\treleaser help %s", e.err, e.cmd.name)
}

func (e *usageError) Unwrap() error {
	return e.err
}

func usageErrorf(cmd *command, format string, args ...interface{}) error {
	return &usageError{cmd: cmd, err: fmt.Errorf(format, args...)}
}
