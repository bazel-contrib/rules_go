// Copyright 2017 The Bazel Authors. All rights reserved.
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

// protoc invokes the protobuf compiler and captures the resulting .pb.go file.
package main

import (
	"bytes"
	"errors"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

type genFileInfo struct {
	base       string       // The basename of the path
	path       string       // The full path to the final file
	expected   bool         // Whether the file is expected by the rules
	created    bool         // Whether the file was created by protoc
	from       *genFileInfo // The actual file protoc produced if not Path
	unique     bool         // True if this base name is unique in expected results
	ambiguious bool         // True if there were more than one possible outputs that matched this file
}

func run(args []string) error {
	// process the args
	args, useParamFile, err := expandParamsFiles(args)
	if err != nil {
		return err
	}
	options := multiFlag{}
	descriptors := multiFlag{}
	expected := multiFlag{}
	imports := multiFlag{}
	flags := flag.NewFlagSet("protoc", flag.ExitOnError)
	protoc := flags.String("protoc", "", "The path to the real protoc.")
	outPath := flags.String("out_path", "", "The base output path to write to.")
	plugin := flags.String("plugin", "", "The go plugin to use.")
	importpath := flags.String("importpath", "", "The importpath for the generated sources.")
	flags.Var(&options, "option", "The plugin options.")
	flags.Var(&descriptors, "descriptor_set", "The descriptor set to read.")
	flags.Var(&expected, "expected", "The expected output files.")
	flags.Var(&imports, "import", "Map a proto file to an import path.")
	if err := flags.Parse(args); err != nil {
		return err
	}

	// Output to a temporary folder and then move the contents into place below.
	// This is to work around long file paths on Windows.
	tmpDir, err := ioutil.TempDir("", "go_proto")
	if err != nil {
		return err
	}
	tmpDir = abs(tmpDir)        // required to work with long paths on Windows
	absOutPath := abs(*outPath) // required to work with long paths on Windows
	defer os.RemoveAll(tmpDir)

	pluginBase := filepath.Base(*plugin)
	pluginName := strings.TrimSuffix(
		strings.TrimPrefix(filepath.Base(*plugin), "protoc-gen-"), ".exe")
	for _, m := range imports {
		options = append(options, fmt.Sprintf("M%v", m))
	}
	if runtime.GOOS == "windows" {
		// Turn the plugin path into raw form, since we're handing it off to a non-go binary.
		// This is required to work with long paths on Windows.
		*plugin = "\\\\?\\" + abs(*plugin)
	}
	protoc_args := []string{
		fmt.Sprintf("--%v_out=%v:%v", pluginName, strings.Join(options, ","), tmpDir),
		"--plugin", fmt.Sprintf("%v=%v", strings.TrimSuffix(pluginBase, ".exe"), *plugin),
		"--descriptor_set_in", strings.Join(descriptors, string(os.PathListSeparator)),
	}
	protoc_args = append(protoc_args, flags.Args()...)

	var cmd *exec.Cmd
	if useParamFile {
		paramFile, err := ioutil.TempFile(tmpDir, "protoc-*.params")
		if err != nil {
			return fmt.Errorf("error creating param file for protoc: %v", err)
		}
		for _, arg := range protoc_args {
			_, err := fmt.Fprintln(paramFile, arg)
			if err != nil {
				return fmt.Errorf("error writing param file for protoc: %v", err)
			}
		}
		cmd = exec.Command(*protoc, "@"+paramFile.Name())
	} else {
		cmd = exec.Command(*protoc, protoc_args...)
	}

	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("error running protoc: %v", err)
	}
	// Build our file map, and test for existance
	files := map[string]*genFileInfo{}
	byBase := map[string]*genFileInfo{}
	for _, path := range expected {
		info := &genFileInfo{
			path:     path,
			base:     filepath.Base(path),
			expected: true,
			unique:   true,
		}
		files[info.path] = info
		if byBase[info.base] != nil {
			info.unique = false
			byBase[info.base].unique = false
		} else {
			byBase[info.base] = info
		}
	}
	// Walk the generated files
	filepath.Walk(tmpDir, func(path string, f os.FileInfo, err error) error {
		relPath, err := filepath.Rel(tmpDir, path)
		if err != nil {
			return err
		}
		if relPath == "." {
			return nil
		}

		if f.IsDir() {
			if err := os.Mkdir(filepath.Join(absOutPath, relPath), f.Mode()); !os.IsExist(err) {
				return err
			}
			return nil
		}

		if !strings.HasSuffix(path, ".go") {
			return nil
		}

		info := &genFileInfo{
			path:    path,
			base:    filepath.Base(path),
			created: true,
		}

		if foundInfo, ok := files[relPath]; ok {
			foundInfo.created = true
			foundInfo.from = info
			return nil
		}
		files[relPath] = info
		copyTo := byBase[info.base]
		switch {
		case copyTo == nil:
			// Unwanted output
		case !copyTo.unique:
			// not unique, no copy allowed
		case copyTo.from != nil:
			copyTo.ambiguious = true
			info.ambiguious = true
		default:
			copyTo.from = info
			copyTo.created = true
			info.expected = true
		}
		return nil
	})
	buf := &bytes.Buffer{}
	for _, f := range files {
		switch {
		case f.expected && !f.created:
			// Some plugins only create output files if the proto source files have
			// have relevant definitions (e.g., services for grpc_gateway). Create
			// trivial files that the compiler will ignore for missing outputs.
			data := []byte("// +build ignore\n\npackage ignore")
			if err := ioutil.WriteFile(abs(f.path), data, 0644); err != nil {
				return err
			}
		case f.expected && f.ambiguious:
			fmt.Fprintf(buf, "Ambiguious output %v.\n", f.path)
		case f.from != nil:
			data, err := ioutil.ReadFile(f.from.path)
			if err != nil {
				return err
			}
			if err := ioutil.WriteFile(abs(f.path), data, 0644); err != nil {
				return err
			}
		case !f.expected:
			//fmt.Fprintf(buf, "Unexpected output %v.\n", f.path)
		}
		if buf.Len() > 0 {
			fmt.Fprintf(buf, "Check that the go_package option is %q.", *importpath)
			return errors.New(buf.String())
		}
	}

	return nil
}

func main() {
	if err := run(os.Args[1:]); err != nil {
		log.Fatal(err)
	}
}
