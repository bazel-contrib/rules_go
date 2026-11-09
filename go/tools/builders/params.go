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

package main

import (
	"fmt"
	"io/ioutil"
	"strings"
)

// expandParamsFiles looks for arguments in args of the form
// "-param=filename". When it finds these arguments it reads the file "filename"
// and replaces the argument with its content.
// It returns the expanded arguments as well as a bool that is true if any param
// files have been passed.
func expandParamsFiles(args []string) ([]string, bool, error) {
	var paramsIndices []int
	for i, arg := range args {
		if strings.HasPrefix(arg, "-param=") {
			paramsIndices = append(paramsIndices, i)
		}
	}
	if len(paramsIndices) == 0 {
		return args, false, nil
	}
	var expandedArgs []string
	last := 0
	for _, pi := range paramsIndices {
		expandedArgs = append(expandedArgs, args[last:pi]...)
		last = pi + 1

		fileName := args[pi][len("-param="):]
		fileArgs, err := readParamsFile(fileName)
		if err != nil {
			return nil, true, err
		}
		expandedArgs = append(expandedArgs, fileArgs...)
	}
	expandedArgs = append(expandedArgs, args[last:]...)
	return expandedArgs, true, nil
}

// readParamsFiles parses a Bazel params file in "shell" format. The file
// should contain one argument per line. Arguments may be quoted with single
// quotes. All characters within quoted strings are interpreted literally
// including newlines and excepting single quotes. Characters outside quoted
// strings may be escaped with a backslash.
func readParamsFile(name string) ([]string, error) {
	data, err := ioutil.ReadFile(name)
	if err != nil {
		return nil, err
	}

	var args []string
	var arg []byte
	quote := false
	escape := false
	for p := 0; p < len(data); p++ {
		b := data[p]
		switch {
		case escape:
			arg = append(arg, b)
			escape = false

		case b == '\'':
			quote = !quote

		case !quote && b == '\\':
			escape = true

		case !quote && b == '\n':
			args = append(args, string(arg))
			arg = arg[:0]

		default:
			arg = append(arg, b)
		}
	}
	if quote {
		return nil, fmt.Errorf("unterminated quote")
	}
	if escape {
		return nil, fmt.Errorf("unterminated escape")
	}
	if len(arg) > 0 {
		args = append(args, string(arg))
	}
	return args, nil
}
