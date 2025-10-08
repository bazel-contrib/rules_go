// Copyright 2022 The Bazel Authors. All rights reserved.
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

package bincov

import (
	"bytes"
	"cmd/internal/cov"
	"fmt"
	"internal/runtime/exithook"
	"log"
	"os"
	"runtime/coverage"

	"github.com/bazelbuild/rules_go/go/tools/bzltestutil"
)

// Lock in the COVERAGE_DIR during test setup in case the test uses e.g. os.Clearenv.
var coverageDir = os.Getenv("COVERAGE_DIR")

func AddExitHook() {
	if coverageDir == "" {
		log.Printf("Not collecting coverage: COVERAGE_DIR is not set")
		return
	}

	// Go 1.24+ binaries built with -cover already emit their own coverage to GOCOVERDIR.
	// We redirect them to a scratch directory that we can ignore.
	// Once we only support these recent versions we could consider using the built-in support.
	// We could also leave this env var unset but that would result in a misleading warning
	// that coverage is not being collected
	ignoredCoverageDir, err := os.MkdirTemp(os.Getenv("TEST_TMPDIR"), "ignored")
	if err != nil {
		fmt.Fprintf(os.Stderr, "creating temp dir for scratch coverage: %v\n", err)
		return
	}
	os.Setenv("GOCOVERDIR", ignoredCoverageDir)

	exithook.Add(exithook.Hook{
		F: func() {
			dir, err := os.MkdirTemp(os.Getenv("TEST_TMPDIR"), "coverage")
			if err != nil {
				fmt.Fprintf(os.Stderr, "create temp dir for coverage: %v\n", err)
				return
			}

			if err := coverage.WriteMetaDir(dir); err != nil {
				fmt.Fprintf(os.Stderr, "write meta: %v\n", err)
				return
			}
			if err := coverage.WriteCountersDir(dir); err != nil {
				fmt.Fprintf(os.Stderr, "write counters: %v\n", err)
				return
			}

			buf := new(bytes.Buffer)
			visitor := makeVisitor(buf)

			verbosityLevel := 0
			var flags cov.CovDataReaderFlags
			reader := cov.MakeCovDataReader(visitor, []string{dir}, verbosityLevel, flags, nil)
			if err := reader.Visit(); err != nil {
				fmt.Fprintf(os.Stderr, "error: %v\n", err)
				return
			}

			if err := bzltestutil.ConvertCoverFromReaderToLcov(buf); err != nil {
				fmt.Fprintf(os.Stderr, "converting to lcov: %v\n", err)
				return
			}
		},
		RunOnFailure: true,
	})
}
