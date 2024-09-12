// Copyright 2018 The Bazel Authors. All rights reserved.
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

// builder implements most of the actions for Bazel to compile and link
// go code. We use a single binary for most actions, since this reduces
// the number of inputs needed for each action and allows us to build
// multiple related files in a single action.

package main

import (
	"log"
	"os"
)

func main() {
	log.SetFlags(0)
	log.SetPrefix("builder: ")

	args, _, err := expandParamsFiles(os.Args[1:])
	if err != nil {
		log.Fatal(err)
	}

	verb := verbFromName(os.Args[0])
	if verb == "" && len(args) == 0 {
		log.Fatalf("usage: %s verb options...", os.Args[0])
	}

	var rest []string
	if verb == "" {
		verb, rest = args[0], args[1:]
	} else {
		rest = args
	}

	var action func(args []string) error
	switch verb {
	case "compilepkg":
		action = compilePkg
	case "nogo":
		action = nogo
	case "nogovalidation":
		action = nogoValidation
	case "filterbuildid":
		action = filterBuildID
	case "gentestmain":
		action = genTestMain
	case "link":
		action = link
	case "gennogomain":
		action = genNogoMain
	case "stdlib":
		action = stdlib
	case "stdliblist":
		action = stdliblist
	case "cc":
		action = cc
	default:
		log.Fatalf("unknown action: %s", verb)
	}
	log.SetPrefix(verb + ": ")

	if err := action(rest); err != nil {
		log.Fatalf("\n$$$$$$$$$$$$$$$$$$$$$$$$ fatal: %+v", err)
	}
}
