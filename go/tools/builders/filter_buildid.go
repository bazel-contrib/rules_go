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

package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"syscall"
)

// filterBuildID executes the tool on the command line, filtering out any
// -buildid arguments. It is intended to be used with -toolexec.
func filterBuildID(args []string) error {
	newArgs := make([]string, 0, len(args))
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if arg == "-buildid" {
			i++
			continue
		}
		newArgs = append(newArgs, arg)
	}
	if isCgoTool(newArgs[0]) {
		// There doesn't seem to be another way to ensure cgo is called with -trimpath here:
		// https://github.com/golang/go/blob/8c445b7c9fe6738cbef2040a1011bd11489b0806/src/cmd/go/internal/work/exec.go#L3288
		newArgs = append([]string{
			newArgs[0],
			"-trimpath", os.Getenv("BAZEL_EXECROOT"),
		}, newArgs[1:]...)
	}
	if runtime.GOOS == "windows" {
		cmd := exec.Command(newArgs[0], newArgs[1:]...)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		return cmd.Run()
	} else {
		return syscall.Exec(newArgs[0], newArgs, os.Environ())
	}
}

func isCgoTool(args0 string) bool {
	basename := filepath.Base(args0)
	if runtime.GOOS == "windows" {
		return basename == "cgo.exe"
	} else {
		return basename == "cgo"
	}
}
