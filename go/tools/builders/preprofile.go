// Copyright 2026 The Bazel Authors. All rights reserved.
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

package main

import (
	"flag"
	"fmt"
	"io"
	"os"
)

// preprofilePrefix starts the header of a profile that has already been
// converted by "go tool preprofile". The version that follows it is not matched:
// the compiler checks the full header itself.
//
// Keep in sync with serializationHeader in cmd/internal/pgo.
const preprofilePrefix = "GO PREPROFILE "

// preprofile converts a pprof profile into the intermediate representation the
// compiler's -pgoprofile flag also accepts, which is far cheaper to parse.
func preprofile(args []string) error {
	flags := flag.NewFlagSet("preprofile", flag.ExitOnError)
	goenv := envFlags(flags)
	in := flags.String("in", "", "Path to the pprof profile to convert")
	out := flags.String("out", "", "Path to the converted profile")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if err := goenv.checkFlagsAndSetGoroot(); err != nil {
		return err
	}
	if *in == "" {
		return fmt.Errorf("-in was not set")
	}
	if *out == "" {
		return fmt.Errorf("-out was not set")
	}

	// "go tool preprofile" fails on its own output, so hand an already converted
	// profile through unchanged.
	converted, err := isPreprofile(*in)
	if err != nil {
		return err
	}
	if converted {
		return copyFile(*in, *out)
	}

	return goenv.runCommand(goenv.goTool("preprofile", "-i", *in, "-o", *out))
}

// isPreprofile reports whether the file at path is the output of
// "go tool preprofile" rather than a raw pprof profile.
func isPreprofile(path string) (bool, error) {
	f, err := os.Open(path)
	if err != nil {
		return false, err
	}
	defer f.Close()

	header := make([]byte, len(preprofilePrefix))
	if _, err := io.ReadFull(f, header); err == io.EOF || err == io.ErrUnexpectedEOF {
		// Too short to carry the header, so it isn't a converted profile.
		return false, nil
	} else if err != nil {
		return false, err
	}
	return string(header) == preprofilePrefix, nil
}
