// Copyright 2021 The Bazel Authors. All rights reserved.
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
	"encoding/json"
	"fmt"
	"io"

	"golang.org/x/tools/go/packages"
)

func ReadDriverRequest(r io.Reader) (*packages.DriverRequest, error) {
	req := &packages.DriverRequest{}
	if err := json.NewDecoder(r).Decode(&req); err != nil {
		return nil, fmt.Errorf("unable to decode driver request: %w", err)
	}
	return req, nil
}
