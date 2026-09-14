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

package bzltestutil

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

// decodedEvent mirrors the fields of event that these tests care about. It is decoded
// separately from event because event's Output field marshals as plain text but has no
// matching UnmarshalText, so json.Unmarshal would otherwise try (and fail) to base64-decode it.
type decodedEvent struct {
	Action string
	Test   string
	Key    string `json:",omitempty"`
	Value  string `json:",omitempty"`
}

// decodeEvents parses newline-delimited JSON events, as produced by Converter, into a slice.
func decodeEvents(t *testing.T, raw []byte) []decodedEvent {
	t.Helper()
	var events []decodedEvent
	for _, line := range bytes.Split(bytes.TrimRight(raw, "\n"), []byte("\n")) {
		if len(line) == 0 {
			continue
		}
		var e decodedEvent
		if err := json.Unmarshal(line, &e); err != nil {
			t.Fatalf("invalid JSON event %q: %v", line, err)
		}
		events = append(events, e)
	}
	return events
}

func findAction(events []decodedEvent, action string) *decodedEvent {
	for i := range events {
		if events[i].Action == action {
			return &events[i]
		}
	}
	return nil
}

func TestConverter_AttrLineBecomesAttrEvent(t *testing.T) {
	input := strings.Join([]string{
		"=== RUN   TestAttr",
		"=== ATTR  TestAttr requirement_id REQ-1234",
		"--- PASS: TestAttr (0.00s)",
		"PASS",
		"",
	}, "\n")
	var out bytes.Buffer
	c := NewConverter(&out, "pkg/testing", 0)

	if _, err := c.Write([]byte(input)); err != nil {
		t.Fatalf("Write: %v", err)
	}
	if err := c.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	events := decodeEvents(t, out.Bytes())
	attr := findAction(events, "attr")
	if attr == nil {
		t.Fatalf("no attr event found in %+v", events)
	}
	if attr.Test != "TestAttr" || attr.Key != "requirement_id" || attr.Value != "REQ-1234" {
		t.Errorf("attr event = %+v, want Test=TestAttr Key=requirement_id Value=REQ-1234", attr)
	}
}

// TestConverter_ThenJSON2XML_AttrReachesProperty exercises both halves of the pipeline:
// raw test stdout -> Converter (test2json.go) -> JSON events -> json2xml (xml.go) -> JUnit
// XML. This is the same round trip Wrap() performs, so it catches the case where json2xml
// handles an "attr" event that the converter never actually emits.
func TestConverter_ThenJSON2XML_AttrReachesProperty(t *testing.T) {
	input := strings.Join([]string{
		"=== RUN   TestAttr",
		"=== ATTR  TestAttr requirement_id REQ-1234",
		"--- PASS: TestAttr (0.00s)",
		"PASS",
		"",
	}, "\n")
	var jsonOut bytes.Buffer
	c := NewConverter(&jsonOut, "pkg/testing", 0)

	if _, err := c.Write([]byte(input)); err != nil {
		t.Fatalf("Write: %v", err)
	}
	if err := c.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	xmlOut, err := json2xml(&jsonOut, "pkg/testing")
	if err != nil {
		t.Fatalf("json2xml: %v", err)
	}

	if !bytes.Contains(xmlOut, []byte(`<property name="requirement_id" value="REQ-1234">`)) {
		t.Errorf("xml output missing requirement_id property, got:\n%s", xmlOut)
	}
}

func TestConverter_AttrLineInSubtestScopesToSubtest(t *testing.T) {
	input := strings.Join([]string{
		"=== RUN   TestAttr",
		"=== RUN   TestAttr/sub",
		"=== ATTR  TestAttr/sub requirement_id REQ-5678",
		"--- PASS: TestAttr/sub (0.00s)",
		"--- PASS: TestAttr (0.00s)",
		"PASS",
		"",
	}, "\n")
	var out bytes.Buffer
	c := NewConverter(&out, "pkg/testing", 0)

	if _, err := c.Write([]byte(input)); err != nil {
		t.Fatalf("Write: %v", err)
	}
	if err := c.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	attr := findAction(decodeEvents(t, out.Bytes()), "attr")
	if attr == nil || attr.Test != "TestAttr/sub" {
		t.Errorf("attr event = %+v, want Test=TestAttr/sub", attr)
	}
}
