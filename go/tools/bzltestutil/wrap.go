// Copyright 2020 The Bazel Authors. All rights reserved.
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
	"bufio"
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"

	"github.com/bazelbuild/rules_go/go/tools/bzltestutil/chdir"
)

// TestWrapperAbnormalExit is used by Wrap to indicate the child
// process exitted without an exit code (for example being killed by a signal).
// We use 6, in line with Bazel's RUN_FAILURE.
const TestWrapperAbnormalExit = 6

func ShouldWrap() bool {
	if wrapEnv, ok := os.LookupEnv("GO_TEST_WRAP"); ok {
		wrap, err := strconv.ParseBool(wrapEnv)
		if err != nil {
			log.Fatalf("invalid value for GO_TEST_WRAP: %q", wrapEnv)
		}
		return wrap
	}
	_, ok := os.LookupEnv("XML_OUTPUT_FILE")
	return ok
}

// shouldAddTestV indicates if the test wrapper should prepend a -test.v flag to
// the test args. This is required to get information about passing tests from
// test2json for complete XML reports.
func shouldAddTestV() bool {
	if wrapEnv, ok := os.LookupEnv("GO_TEST_WRAP_TESTV"); ok {
		wrap, err := strconv.ParseBool(wrapEnv)
		if err != nil {
			log.Fatalf("invalid value for GO_TEST_WRAP_TESTV: %q", wrapEnv)
		}
		return wrap
	}
	return false
}

// streamMerger intelligently merges an input stdout and stderr stream and dumps
// the output to the writer `inner`. Additional synchronization is applied to
// ensure that one line at a time is written to the inner writer.
type streamMerger struct {
	OutW, ErrW *io.PipeWriter
	mutex      sync.Mutex
	inner      io.Writer
	wg         sync.WaitGroup
	outR, errR *bufio.Reader
}

func NewStreamMerger(w io.Writer) *streamMerger {
	outR, outW := io.Pipe()
	errR, errW := io.Pipe()
	return &streamMerger{
		inner: w,
		OutW:  outW,
		ErrW:  errW,
		outR:  bufio.NewReader(outR),
		errR:  bufio.NewReader(errR),
	}
}

func (m *streamMerger) Start() {
	m.wg.Add(2)
	process := func(r *bufio.Reader) {
		for {
			s, err := r.ReadString('\n')
			if len(s) > 0 {
				m.mutex.Lock()
				io.WriteString(m.inner, s)
				m.mutex.Unlock()
			}
			if err == io.EOF {
				break
			}
		}
		m.wg.Done()
	}
	go process(m.outR)
	go process(m.errR)
}

func (m *streamMerger) Wait() {
	m.wg.Wait()
}

func Wrap(pkg string) error {
	var jsonBuffer bytes.Buffer
	jsonConverter := NewConverter(&jsonBuffer, pkg, Timestamp)
	streamMerger := NewStreamMerger(jsonConverter)

	args := os.Args[1:]
	if shouldAddTestV() {
		// The -test.v=test2json flag is like -test.v=true but causes the test to add
		// extra ^V characters before testing output lines and other framing,
		// which helps test2json do a better job creating the JSON events.
		args = append([]string{"-test.v=test2json"}, args...)
	}
	exePath := os.Args[0]
	if !filepath.IsAbs(exePath) && strings.ContainsRune(exePath, filepath.Separator) && chdir.TestExecDir != "" {
		exePath = filepath.Join(chdir.TestExecDir, exePath)
	}

	// If Bazel sends a SIGTERM because the test timed out, it sends it to all child processes. However,
	// we want the wrapper to be around to capute and forward the test output when this happens. Thus,
	// we need to ignore the signal. This wrapper will natually ends after the Go test ends, either by
	// SIGTERM or the time set by -test.timeout expires. If that doesn't happen, the test and this warpper
	// will be killed by Bazel after the grace period (15s) expires.
	signal.Ignore(syscall.SIGTERM)

	cmd := exec.Command(exePath, args...)
	cmd.Env = append(os.Environ(), "GO_TEST_WRAP=0")
	cmd.Stderr = io.MultiWriter(os.Stderr, streamMerger.ErrW)
	cmd.Stdout = io.MultiWriter(os.Stdout, streamMerger.OutW)
	streamMerger.Start()
	err := cmd.Run()
	streamMerger.ErrW.Close()
	streamMerger.OutW.Close()
	streamMerger.Wait()
	if err != nil {
		// force jsonConverter to flush the buffer, so we get the "fail" event when a test case panics.
		jsonConverter.Write([]byte{marker})
		jsonConverter.Write(bigFail)
		jsonConverter.Write([]byte("\n"))
	}
	jsonConverter.Close()
	if out, ok := os.LookupEnv("XML_OUTPUT_FILE"); ok {
		werr := writeReport(jsonBuffer, pkg, out)
		if werr != nil {
			if err != nil {
				return fmt.Errorf("error while generating testreport: %s, (error wrapping test execution: %s)", werr, err)
			}
			return fmt.Errorf("error while generating testreport: %s", werr)
		}
	}
	return err
}

func writeReport(jsonBuffer bytes.Buffer, pkg string, path string) error {
	xml, cerr := json2xml(&jsonBuffer, pkg)
	if cerr != nil {
		return fmt.Errorf("error converting test output to xml: %s", cerr)
	}
	if err := ioutil.WriteFile(path, xml, 0664); err != nil {
		return fmt.Errorf("error writing test xml: %s", err)
	}
	return nil
}
