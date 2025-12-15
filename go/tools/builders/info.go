// Copyright 2017 The Bazel Authors. All rights reserved.
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

// info prints debugging information about the go environment.
// It is used to help examine the execution environment of rules_go
package main

import (
	"flag"
	"fmt"
	"log"
	"os"
)

func run(args []string) error {
	args, _, err := expandParamsFiles(args)
	if err != nil {
		return err
	}
	filename := ""
	flags := flag.NewFlagSet("info", flag.ExitOnError)
	flags.StringVar(&filename, "out", filename, "The file to write the report to")
	goenv := envFlags(flags)
	if err := flags.Parse(args); err != nil {
		return err
	}
	if err := goenv.checkFlagsAndSetGoroot(); err != nil {
		return err
	}
	os.Setenv("GO111MODULE", "off")
	f := os.Stderr
	if filename != "" {
		var err error
		f, err = os.Create(filename)
		if err != nil {
			return fmt.Errorf("Could not create report file: %v", err)
		}
		defer f.Close()
	}
	if err := goenv.runCommandToFile(f, os.Stderr, goenv.goCmd("version")); err != nil {
		return err
	}
	if err := goenv.runCommandToFile(f, os.Stderr, goenv.goCmd("env")); err != nil {
		return err
	}
	return nil
}

func main() {
	if err := run(os.Args[1:]); err != nil {
		log.Fatal(err)
	}
}
