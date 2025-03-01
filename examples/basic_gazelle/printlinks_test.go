// Copyright 2025 The Bazel Authors. All rights reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package main

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"golang.org/x/net/html"
)

func TestParseAndGetLinks(t *testing.T) {
	r := strings.NewReader(`<!DOCTYPE html>
<html>
  <head>
	  <title>example</title>
	</head>
	<body>
	  <a href="example.com/a">A</a><br>
		<a href="example.com/b">B</a>
	</body>
</html>
`)
	node, err := html.Parse(r)
	if err != nil {
		t.Fatal(err)
	}
	links := getLinks(node)
	assert.Equal(t, links, []string{"example.com/a", "example.com/b"})
}
