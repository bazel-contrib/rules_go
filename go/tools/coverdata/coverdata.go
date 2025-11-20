/* Copyright 2018 The Bazel Authors. All rights reserved.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

   http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

// Package coverdata provides a registration function for files with
// coverage instrumentation.
//
// This package is part of the Bazel Go rules, and its interface
// should not be considered public. It may change without notice.
package coverdata

import (
	"fmt"
	"testing"
)

// Contains all coverage data for the program.
var (
	Counters = make(map[string][]uint32)
	Blocks = make(map[string][]testing.CoverBlock)

	SrcPathMapping = make(map[string]string)
)

// RegisterFile causes the coverage data recorded for a file to be included
// in program-wide coverage reports. This should be called from init functions
// in packages with coverage instrumentation.
func RegisterFile(fileName string, counter []uint32, pos []uint32, numStmts []uint16) {
	if 3*len(counter) != len(pos) || len(counter) != len(numStmts) {
		panic("coverage: mismatched sizes")
	}
	if Counters[fileName] != nil {
		// Already registered.
		fmt.Printf("Already covered %s\n", fileName)
		return
	}
	Counters[fileName] = counter
	block := make([]testing.CoverBlock, len(counter))
	for i := range counter {
		block[i] = testing.CoverBlock{
			Line0: pos[3*i+0],
			Col0:  uint16(pos[3*i+2]),
			Line1: pos[3*i+1],
			Col1:  uint16(pos[3*i+2] >> 16),
			Stmts: numStmts[i],
		}
	}
	Blocks[fileName] = block
}

func RegisterSrcPathMapping(importPathFile string, srcName string) {
	SrcPathMapping[importPathFile] = srcName
}
