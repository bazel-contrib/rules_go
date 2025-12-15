// Copyright 2024 The Bazel Authors. All rights reserved.
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

package bzltestutil

import (
	"os"
	"os/signal"
	"syscall"
)

func RegisterTimeoutHandler() {
	// If Bazel sends a SIGTERM because the test timed out, it sends it to all child processes. Because
	// we set -test.timeout according to the TEST_TIMEOUT, we need to ignore the signal so the test has
	// time to properly produce the output (e.g. stack trace). It will be killed by Bazel after the grace
	// period (15s) expires.
	//
	// If TEST_TIMEOUT is not set (e.g., when the test binary is run by Delve for debugging), we don't
	// ignore SIGTERM so it can be properly terminated. (1)
	// We do not panic (like native go test does) because users may legitimately want to use SIGTERM
	// in tests.
	//
	// signal.Notify is used to ensure that there is a no-op signal handler registered.
	// Avoid using signal.Ignore here: despite the name, it's only used to unregister handlers that
	// were previously registered by signal.Notify. See (2) for more information.
	//
	// (1): https://github.com/golang/go/blob/e816eb50140841c524fd07ecb4eaa078954eb47c/src/testing/testing.go#L2351
	// (2): https://github.com/bazelbuild/rules_go/pull/3929
	c := make(chan os.Signal, 1)
	signal.Notify(c, syscall.SIGTERM)
}
