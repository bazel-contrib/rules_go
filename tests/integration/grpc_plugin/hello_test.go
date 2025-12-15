// Copyright 2023 The Bazel Authors. All rights reserved.
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

package hello_test

import (
	"reflect"
	"testing"

	"github.com/bazelbuild/rules_go/tests/integration/grpc_plugin/hello"
	pb "github.com/bazelbuild/rules_go/tests/integration/grpc_plugin/hello_proto"
	"google.golang.org/grpc"
)

func Test_ServiceRegistrar(t *testing.T) {
	fr := &fakeRegistrar{}
	pb.RegisterGreetServer(fr, hello.GreetServer())
	if got, want := fr.services, []string{"rules_go.tests.integration.grpc_plugin.Greet"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}

type fakeRegistrar struct {
	services []string
}

func (fr *fakeRegistrar) RegisterService(desc *grpc.ServiceDesc, impl any) {
	fr.services = append(fr.services, desc.ServiceName)
}
