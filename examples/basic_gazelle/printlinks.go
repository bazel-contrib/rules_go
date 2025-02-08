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

// printlinks is a small Go program that reads HTML on stdin and prints
// URLs from <a> tags.
//
// The code isn't really important. It's just meant to show to use build a
// Go project that depends on a package outside the standard library.
package main

import (
	"fmt"
	"os"

	"golang.org/x/net/html"
	"golang.org/x/net/html/atom"
)

func main() {
	node, err := html.Parse(os.Stdin)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
	for _, link := range getLinks(node) {
		fmt.Println(link)
	}
}

func getLinks(root *html.Node) []string {
	var links []string
	for node := range root.Descendants() {
		if node.Type == html.ElementNode && node.DataAtom == atom.A {
			for _, attr := range node.Attr {
				if attr.Key == "href" {
					links = append(links, attr.Val)
				}
			}
		}
	}
	return links
}
