package main

import (
	"encoding/json"
	"flag"
	"log"
	"os"
	"strconv"
	"strings"
	"go/parser"
	"go/token"
	"path/filepath"
)

var (
	bazelBin              = getenvDefault("GOPACKAGESDRIVER_BAZEL", "bazel")
	bazelStartupFlags     = strings.Fields(os.Getenv("GOPACKAGESDRIVER_BAZEL_FLAGS"))
	bazelCommonFlags      = strings.Fields(os.Getenv("GOPACKAGESDRIVER_BAZEL_COMMON_FLAGS"))
	workspaceRoot         = os.Getenv("BUILD_WORKSPACE_DIRECTORY")
	buildWorkingDirectory = os.Getenv("BUILD_WORKING_DIRECTORY")
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		log.Fatalf("write package json: %v", err)
	}
}

type params struct {
	id string
	pkgPath string
	exportFile string
	goFiles []string
	compiledGoFiles []string
	otherFiles []string
	imports map[string]string
	cgoOutDir string
	// path to write the pkgjson
	output string
}

type pkgJson struct {
	ID string `json:"ID"`
	PkgPath string `json:"PkgPath"`
	ExportFile string `json:"ExportFile"`
	GoFiles []string `json:"GoFiles"`
	CompiledGoFiles []string `json:"CompiledGoFiles"`
	OtherFiles []string `json:"OtherFiles"`
	Imports map[string]string `json:"Imports"`
}

func parseArgs(args []string) (*params, error) {
	var params params

	fs := flag.NewFlagSet("pkgjson", flag.ContinueOnError)
	fs.StringVar(&params.id, "id", "", "id")
	fs.StringVar(&params.pkgPath, "pkg_path", "", "package path")
	fs.StringVar(&params.exportFile, "export_file", "", "export file")
	fs.StringVar(&params.cgoOutDir, "cgo_out_dir", "", "cgo out dir")
	rawGoFiles := fs.String("go_files", "", "a comma separated list of go files")
	rawCompiledGoFiles := fs.String("compiled_go_files", "", "a comma separated list of compiled go files")
	rawOtherFiles := fs.String("other_files", "", "a comma separated list of other files")
	rawImports := fs.String("imports", "", "comma separate pairs of importpath=label")
	fs.StringVar(&params.output, "output", "", "path to write the pkgjson")

	err := fs.Parse(args)
	if err != nil {
		return nil, err
	}

	params.goFiles = strings.Split(*rawGoFiles, ",")
	params.compiledGoFiles = strings.Split(*rawCompiledGoFiles, ",")
	params.otherFiles = strings.Split(*rawOtherFiles, ",")
	params.imports = make(map[string]string)
	if len(*rawImports) > 0 {
		for _, rawImport := range strings.Split(*rawImports, ",") {
			parts := strings.Split(rawImport, "=")
			params.imports[parts[0]] = parts[1]
		}
	}

	return &params, nil
}

func run(args []string) error {
	params, err := parseArgs(args)
	if err != nil {
		return err
	}
	data := pkgJson{
		ID: params.id,
		PkgPath: params.pkgPath,
		ExportFile: params.exportFile,
		GoFiles: params.goFiles,
		CompiledGoFiles: params.compiledGoFiles,
		OtherFiles: params.otherFiles,
		Imports: params.imports,
	}
	if err = processCgoFiles(params.cgoOutDir, &data, resolvePath); err != nil {
		return err
	}
	marshaled, err := json.Marshal(data)
	if err != nil {
		return err
	}
	return os.WriteFile(params.output, marshaled, 0644)
}

// go/tools/gopackagesdriver uses special Bazel file prefixes to resolve paths
// in package json files to absolute paths. This function strips these prefixes
// which returns paths relative to the execroot.
// The prefixes are defined in:
// github.com/bazel-contrib/rules_go/go/tools/gopackagesdriver/pkgjson/pkg_json.bzl
func resolvePath(p string) string {
	if strings.HasPrefix(p, "__BAZEL_OUTPUT_BASE__") {
		return strings.TrimPrefix(p, "__BAZEL_OUTPUT_BASE__/")
	}
	if strings.HasPrefix(p, "__BAZEL_EXECROOT__") {
		return strings.TrimPrefix(p, "__BAZEL_EXECROOT__/")
	}
	if strings.HasPrefix(p, "__BAZEL_WORKSPACE__") {
		return strings.TrimPrefix(p, "__BAZEL_WORKSPACE__/")
	}
	return p
}

// processCgoFiles ensures that the CompiledGoFiles field of the package json does not contain
// any cgo source files and contains cgo generated files.
func processCgoFiles(cgoOutDir string, p *pkgJson, pathResolver func(string) string) error {
	cf, err := filterCgoSourceFiles(p.CompiledGoFiles, pathResolver)
	if err != nil {
		return err
	}
	p.CompiledGoFiles = cf.goSourceFiles
	if cgoOutDir == "" {
		return nil
	}
	cgoSrcs, err := cgoGoSrcs(cgoOutDir, cf.cgoSourceFiles, pathResolver)
	if err != nil {
		return err
	}
	p.CompiledGoFiles = append(p.CompiledGoFiles, cgoSrcs...)
	return nil
}

type cgoFiles struct {
	cgoSourceFiles []string
	goSourceFiles []string
}

func filterCgoSourceFiles(compiledGoFiles []string, resolvePath func(string) string) (cgoFiles, error) {
	filtered := make([]string, 0, len(compiledGoFiles))
	var cgoSourceFiles []string
	fset := token.NewFileSet()
	for _, file := range compiledGoFiles {
		// Note: We check resolved file paths to get a path relative to the execroot
		// but we add the unresolved file path to the package json as the gopackagesdriver
		// uses the special bazel prefixes to resolve to absolute paths.
		resolvedPath := resolvePath(file)
		f, err := parser.ParseFile(fset, resolvedPath, nil, parser.ImportsOnly)
		if err != nil {
			return cgoFiles{}, err
		}
		var skip bool
		for _, rawImport := range f.Imports {
			imp, err := strconv.Unquote(rawImport.Path.Value)
			if err != nil {
				continue
			}
			if imp == "C" {
				skip = true
				break
			}
		}
		if skip {
			// Skip Cgo preprocessed files.
			cgoSourceFiles = append(cgoSourceFiles, file)
		} else {
			filtered = append(filtered, file)
		}
	}
	return cgoFiles{
		cgoSourceFiles: cgoSourceFiles,
		goSourceFiles:  filtered,
	}, nil
}

func cgoGoSrcs(cgoGeneratedDir string, cgoSourceFiles []string, resolvePath func(string) string) ([]string, error) {
	var cgoGeneratedFiles []string
	// Only include cgo generated files if they were generated by Bazel, using
	// the _cgo_gotypes.go file name as a marker for cgo files as it's always
	// present if the package contains cgo generated code.
	//
	// Note: We check resolved file paths to get a path relative to the execroot
	// but we add the unresolved file path to the package json as the gopackagesdriver
	// uses the special bazel prefixes to resolve to absolute paths.
	resolvedCgoGeneratedDir := resolvePath(cgoGeneratedDir)
	cgoGotypesPath := filepath.Join(resolvedCgoGeneratedDir, "_cgo_gotypes.go")
	if _, err := os.Stat(cgoGotypesPath); err != nil && os.IsNotExist(err) {
		return nil, nil
	}
	cgoGeneratedFiles = append(
		cgoGeneratedFiles,
		filepath.Join(cgoGeneratedDir, "_cgo_gotypes.go"),
		filepath.Join(cgoGeneratedDir, "_cgo_imports.go"),
	)
	for _, csf := range cgoSourceFiles {
		name := strings.TrimSuffix(filepath.Base(csf), ".go")
		name = name + ".cgo1.go"
		resolvedPath := filepath.Join(resolvedCgoGeneratedDir, name)
		unresolvedPath := filepath.Join(cgoGeneratedDir, name)
		if _, err := os.Stat(resolvedPath); err != nil && os.IsNotExist(err) {
			// Check if the cgo generated file exists. Due to situations
			// like OS-specific build tags, we can't determine whether a cgo
			// processed file is generated ahead of time.
			continue
		} else {
			cgoGeneratedFiles = append(cgoGeneratedFiles, unresolvedPath)
		}
	}
	return cgoGeneratedFiles, nil
}

// getenvDefault returns the value of the environment variable key.
// If the variable is not set, it returns defaultValue.
func getenvDefault(key, defaultValue string) string {
	if v, ok := os.LookupEnv(key); ok {
		return v
	}
	return defaultValue
}
