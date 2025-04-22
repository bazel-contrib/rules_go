/* Copyright 2018 The Bazel Authors. All rights reserved.

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

// Generates the nogo binary to analyze Go source code at build time.

package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io/ioutil"
	"math"
	"os"
	"regexp"
	"strconv"
	"text/template"
)

const nogoMainTpl = `
package main


import (
{{- if .NeedRegexp }}
	"regexp"
{{- end}}
{{- range $import := .Imports}}
	{{$import.Name}} "{{$import.Path}}"
{{- end}}
	"golang.org/x/tools/go/analysis"
)

var analyzers = []*analysis.Analyzer{
{{- range $import := .Imports}}
	{{$import.Name}}.Analyzer,
{{- end}}
}

// configs maps analysis names to configurations.
var configs = map[string]config{
{{- range $name, $config := .Configs}}
	{{printf "%q" $name}}: config{
		{{- if $config.AnalyzerFlags }}
		analyzerFlags: map[string]string {
			{{- range $flagKey, $flagValue := $config.AnalyzerFlags}}
			{{printf "%q: %q" $flagKey $flagValue}},
			{{- end}}
		},
		{{- end -}}
		{{- if $config.OnlyFiles}}
		onlyFiles: []*regexp.Regexp{
			{{- range $path, $comment := $config.OnlyFiles}}
			{{- if $comment}}
			// {{$comment}}
			{{end -}}
			{{printf "regexp.MustCompile(%q)" $path}},
			{{- end}}
		},
		{{- end -}}
		{{- if $config.ExcludeFiles}}
		excludeFiles: []*regexp.Regexp{
			{{- range $path, $comment := $config.ExcludeFiles}}
			{{- if $comment}}
			// {{$comment}}
			{{end -}}
			{{printf "regexp.MustCompile(%q)" $path}},
			{{- end}}
		},
		{{- end}}
	},
{{- end}}
}

const debugMode = {{ .Debug }}
`

func genNogoMain(args []string) error {
	analyzerImportPaths := multiFlag{}
	flags := flag.NewFlagSet("generate_nogo_main", flag.ExitOnError)
	out := flags.String("output", "", "output file to write (defaults to stdout)")
	flags.Var(&analyzerImportPaths, "analyzer_importpath", "import path of an analyzer library")
	configFile := flags.String("config", "", "nogo config file")
	debug := flags.Bool("debug", false, "enable debug mode")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if *out == "" {
		return errors.New("must provide output file")
	}

	outFile := os.Stdout
	var cErr error
	outFile, err := os.Create(*out)
	if err != nil {
		return fmt.Errorf("os.Create(%q): %v", *out, err)
	}
	defer func() {
		if err := outFile.Close(); err != nil {
			cErr = fmt.Errorf("error closing %s: %v", outFile.Name(), err)
		}
	}()

	config, err := buildConfig(*configFile)
	if err != nil {
		return err
	}

	type Import struct {
		Path, Name string
	}
	// Create unique name for each imported analyzer.
	suffix := 1
	imports := make([]Import, 0, len(analyzerImportPaths))
	for _, path := range analyzerImportPaths {
		imports = append(imports, Import{
			Path: path,
			Name: "analyzer" + strconv.Itoa(suffix)})
		if suffix == math.MaxInt32 {
			return fmt.Errorf("cannot generate more than %d analyzers", suffix)
		}
		suffix++
	}
	data := struct {
		Imports    []Import
		Configs    Configs
		NeedRegexp bool
		Debug      bool
	}{
		Imports: imports,
		Configs: config,
		Debug:   *debug,
	}
	for _, c := range config {
		if len(c.OnlyFiles) > 0 || len(c.ExcludeFiles) > 0 {
			data.NeedRegexp = true
			break
		}
	}

	tpl := template.Must(template.New("source").Parse(nogoMainTpl))
	if err := tpl.Execute(outFile, data); err != nil {
		return fmt.Errorf("template.Execute failed: %v", err)
	}
	return cErr
}

func buildConfig(path string) (Configs, error) {
	if path == "" {
		return Configs{}, nil
	}
	b, err := ioutil.ReadFile(path)
	if err != nil {
		return Configs{}, fmt.Errorf("failed to read config file: %v", err)
	}
	configs := make(Configs)
	if err = json.Unmarshal(b, &configs); err != nil {
		return Configs{}, fmt.Errorf("failed to unmarshal config file: %v", err)
	}
	for name, config := range configs {
		for pattern := range config.OnlyFiles {
			if _, err := regexp.Compile(pattern); err != nil {
				return Configs{}, fmt.Errorf("invalid pattern for analysis %q: %v", name, err)
			}
		}
		for pattern := range config.ExcludeFiles {
			if _, err := regexp.Compile(pattern); err != nil {
				return Configs{}, fmt.Errorf("invalid pattern for analysis %q: %v", name, err)
			}
		}
		configs[name] = Config{
			// Description is currently unused.
			OnlyFiles:     config.OnlyFiles,
			ExcludeFiles:  config.ExcludeFiles,
			AnalyzerFlags: config.AnalyzerFlags,
		}
	}
	return configs, nil
}

type Configs map[string]Config

type Config struct {
	Description   string
	OnlyFiles     map[string]string `json:"only_files"`
	ExcludeFiles  map[string]string `json:"exclude_files"`
	AnalyzerFlags map[string]string `json:"analyzer_flags"`
}
