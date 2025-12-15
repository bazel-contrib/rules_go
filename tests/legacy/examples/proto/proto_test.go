/* Copyright 2017 The Bazel Authors. All rights reserved.

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

package proto

import (
	"testing"

	embed "github.com/bazelbuild/rules_go/examples/proto/embed"
	lib_proto "github.com/bazelbuild/rules_go/examples/proto/lib/lib_proto"
)

func TestProto(t *testing.T) {
	p := lib_proto.LibObject{AreYouSure: 20}
	sure := p.GetAreYouSure()
	if sure != 20 {
		t.Errorf("got %d, want 20", sure)
	}
}

func TestEmbed(t *testing.T) {
	if embed.OtherThing().A != 42 {
		t.Errorf("Unable to call method from embedded go files")
	}
}
