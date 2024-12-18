package main

import (
	"fmt"
	"go/token"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"golang.org/x/tools/go/analysis"
)

// diagnosticEntry represents a diagnostic entry with the corresponding analyzer.
type diagnosticEntry struct {
	analysis.Diagnostic
	*analysis.Analyzer
}

// This file contains two main entities: Edit and Change, which correspond to the low-level
// and high-level abstractions. See them below.

// The following is about the `Edit`, a low-level abstraction of edits.
// An Edit describes the replacement of a portion of a text file.
type Edit struct {
	New   string `json:"new"`   // the replacement
	Start int    `json:"start"` // starting byte offset of the region to replace
	End   int    `json:"end"`   // (exclusive) ending byte offset of the region to replace
}


// FileEdits represents the mapping of analyzers to their edits for a specific file.
type FileEdits struct {
	AnalyzerToEdits map[string][]Edit `json:"analyzer_to_edits"` // Analyzer as the key, edits as the value
}

// Change represents a collection of file edits.
type Change struct {
	FileToEdits map[string]FileEdits `json:"file_to_edits"` // File path as the key, analyzer-to-edits mapping as the value
}

// NewChange creates a new Change object.
func NewChange() *Change {
	return &Change{
		FileToEdits: make(map[string]FileEdits),
	}
}

func (e Edit) String() string {
	return fmt.Sprintf("{Start:%d,End:%d,New:%q}", e.Start, e.End, e.New)
}

// SortEdits orders a slice of Edits by (start, end) offset.
// This ordering puts insertions (end = start) before deletions
// (end > start) at the same point, but uses a stable sort to preserve
// the order of multiple insertions at the same point.
// (Apply detects multiple deletions at the same point as an error.)
func SortEdits(edits []Edit) {
	sort.Stable(editsSort(edits))
}

type editsSort []Edit

func (a editsSort) Len() int { return len(a) }
func (a editsSort) Less(i, j int) bool {
	if cmp := a[i].Start - a[j].Start; cmp != 0 {
		return cmp < 0
	}
	return a[i].End < a[j].End
}
func (a editsSort) Swap(i, j int) { a[i], a[j] = a[j], a[i] }

// UniqueEdits returns a list of edits that is sorted and
// contains no duplicate edits. Returns the index of some
// overlapping adjacent edits if there is one and <0 if the
// edits are valid.
// Deduplication helps in the cases where two analyzers produce duplicate edits.
func UniqueEdits(edits []Edit) ([]Edit, int) {
	if len(edits) == 0 {
		return nil, -1
	}
	equivalent := func(x, y Edit) bool {
		return x.Start == y.Start && x.End == y.End && x.New == y.New
	}
	SortEdits(edits)
	unique := []Edit{edits[0]}
	invalid := -1
	for i := 1; i < len(edits); i++ {
		prev, cur := edits[i-1], edits[i]
		if !equivalent(prev, cur) {
			unique = append(unique, cur)
			if prev.End > cur.Start {
				invalid = i
			}
		}
	}
	return unique, invalid
}

// ApplyEditsBytes applies a sequence of edits to the src byte slice and returns the result.
// Edits are applied in order of start offset; edits with the same start offset are applied in the order they were provided.
// ApplyEditsBytes returns an error if any edit is out of bounds, or if any pair of edits is overlapping.
func ApplyEditsBytes(src []byte, edits []Edit) ([]byte, error) {
	// Validate and compute the output size based on the edits.
	edits, size, err := validateBytes(src, edits)
	if err != nil {
		return nil, err
	}

	// Apply the edits.
	out := make([]byte, 0, size)
	lastEnd := 0
	for _, edit := range edits {
		if lastEnd < edit.Start {
			out = append(out, src[lastEnd:edit.Start]...)
		}
		out = append(out, edit.New...)
		lastEnd = edit.End
	}
	out = append(out, src[lastEnd:]...)

	if len(out) != size {
		panic("wrong size")
	}

	return out, nil
}

// validateBytes checks that edits are consistent with the src byte slice,
// and returns the size of the patched output. It may return a different slice if edits are sorted.
func validateBytes(src []byte, edits []Edit) ([]Edit, int, error) {
	if !sort.IsSorted(editsSort(edits)) {
		edits = append([]Edit(nil), edits...)
		SortEdits(edits)
	}

	// Check validity of edits and compute final size.
	size := len(src)
	lastEnd := 0
	for _, edit := range edits {
		if !(0 <= edit.Start && edit.Start <= edit.End && edit.End <= len(src)) {
			return nil, 0, fmt.Errorf("diff has out-of-bounds edits")
		}
		if edit.Start < lastEnd {
			return nil, 0, fmt.Errorf("diff has overlapping edits")
		}
		size += len(edit.New) + edit.Start - edit.End
		lastEnd = edit.End
	}

	return edits, size, nil
}


// NewChangeFromDiagnostics builds a Change from a set of diagnostics.
// Unlike Diagnostic, Change is independent of the FileSet given it uses perf-file offsets instead of token.Pos.
// This allows Change to be used in contexts where the FileSet is not available, e.g., it remains applicable after it is saved to disk and loaded back.
// See https://github.com/golang/tools/blob/master/go/analysis/diagnostic.go for details.
func NewChangeFromDiagnostics(entries []diagnosticEntry, fileSet *token.FileSet) (*Change, error) {
	c := NewChange()

	cwd, err := os.Getwd()
	if err != nil {
		return c, fmt.Errorf("Error getting current working directory: (%v)", err)
	}

	var allErrors []error

	for _, entry := range entries {
		analyzer := entry.Analyzer.Name
		for _, sf := range entry.Diagnostic.SuggestedFixes {
			for _, edit := range sf.TextEdits {
				start, end := edit.Pos, edit.End
				if !end.IsValid() {
					end = start
				}

				file := fileSet.File(edit.Pos)
				if file == nil {
					allErrors = append(allErrors, fmt.Errorf("invalid fix: missing file info for pos %v", edit.Pos))
					continue
				}
				if start > end {
					allErrors = append(allErrors, fmt.Errorf("invalid fix: pos %v > end %v", start, end))
					continue
				}
				if eof := token.Pos(file.Base() + file.Size()); end > eof {
					allErrors = append(allErrors, fmt.Errorf("invalid fix: end %v past end of file %v", end, eof))
					continue
				}

				edit := Edit{Start: file.Offset(start), End: file.Offset(end), New: string(edit.NewText)}
				fileRelativePath, err := filepath.Rel(cwd, file.Name())
				if err != nil {
					fileRelativePath = file.Name() // fallback logic
				}
				c.AddEdit(fileRelativePath, analyzer, edit)
			}
		}
	}

	if len(allErrors) > 0 {
		return c, fmt.Errorf("errors: %v", allErrors)
	}
	return c, nil
}


// AddEdit adds an edit to the Change, organizing by file and analyzer.
func (c *Change) AddEdit(file string, analyzer string, edit Edit) {
	// Ensure the FileEdits structure exists for the file
	fileEdits, exists := c.FileToEdits[file]
	if !exists {
		fileEdits = FileEdits{
			AnalyzerToEdits: make(map[string][]Edit),
		}
		c.FileToEdits[file] = fileEdits
	}

	// Append the edit to the list of edits for the analyzer
	fileEdits.AnalyzerToEdits[analyzer] = append(fileEdits.AnalyzerToEdits[analyzer], edit)
}



// Flatten merges all edits for a file from different analyzers into a single map of file-to-edits.
// Edits from each analyzer are processed in a deterministic order, and overlapping edits are skipped.
func Flatten(change Change) map[string][]Edit {
	fileToEdits := make(map[string][]Edit)

	for file, fileEdits := range change.FileToEdits {
		// Get a sorted list of analyzers for deterministic processing order
		analyzers := make([]string, 0, len(fileEdits.AnalyzerToEdits))
		for analyzer := range fileEdits.AnalyzerToEdits {
			analyzers = append(analyzers, analyzer)
		}
		sort.Strings(analyzers)

		mergedEdits := make([]Edit, 0)

		for _, analyzer := range analyzers {
			edits := fileEdits.AnalyzerToEdits[analyzer]

			// Deduplicate and sort edits for the current analyzer
			edits, _ = UniqueEdits(edits)

			// Merge edits into the current list, checking for overlaps
			candidateEdits := append(mergedEdits, edits...)
			candidateEdits, invalidIndex := UniqueEdits(candidateEdits)
			if invalidIndex >= 0 {
				// Skip edits from this analyzer if merging them would cause overlaps.
				// Apply the non-overlapping edits first. After that, a rerun of bazel build will
				// allows these skipped edits to be applied separately.
				// Note the resolution happens to each file independently.
				continue
			}

			// Update the merged edits
			mergedEdits = candidateEdits
		}

		// Store the final merged edits for the file
		fileToEdits[file] = mergedEdits
	}

	return fileToEdits
}


// ToCombinedPatch converts all edits to a single consolidated patch.
func ToCombinedPatch(fileToEdits map[string][]Edit) (string, error) {
	var combinedPatch strings.Builder

	filePaths := make([]string, 0, len(fileToEdits))
	for filePath := range fileToEdits {
		filePaths = append(filePaths, filePath)
	}
	sort.Strings(filePaths) // Sort file paths alphabetically

	// Iterate over sorted file paths
	for _, filePath := range filePaths {
		edits := fileToEdits[filePath]
		if len(edits) == 0 {
			continue
		}

		// Ensure edits are unique and sorted
		edits, _ = UniqueEdits(edits)
		contents, err := os.ReadFile(filePath)
		if err != nil {
			return "", fmt.Errorf("failed to read file %s: %v", filePath, err)
		}

		out, err := ApplyEditsBytes(contents, edits)
		if err != nil {
			return "", fmt.Errorf("failed to apply edits for file %s: %v", filePath, err)
		}

		diff := UnifiedDiff{
			A:        trimWhitespaceHeadAndTail(SplitLines(string(contents))),
			B:        trimWhitespaceHeadAndTail(SplitLines(string(out))),
			FromFile: fmt.Sprintf("a/%s", filePath),
			ToFile:   fmt.Sprintf("b/%s", filePath),
			Context:  3,
		}

		patch, err := GetUnifiedDiffString(diff)
		if err != nil {
			return "", fmt.Errorf("failed to generate patch for file %s: %v", filePath, err)
		}

		// Append the patch for this file to the giant patch
		combinedPatch.WriteString(patch)
		combinedPatch.WriteString("\n") // Ensure separation between file patches
	}

	// Remove trailing newline
	result := combinedPatch.String()
	if len(result) > 0 && result[len(result)-1] == '\n' {
		result = result[:len(result)-1]
	}

	return result, nil
}


func trimWhitespaceHeadAndTail(lines []string) []string {
	// Trim left
	for len(lines) > 0 && strings.TrimSpace(lines[0]) == "" {
		lines = lines[1:]
	}

	// Trim right
	for len(lines) > 0 && strings.TrimSpace(lines[len(lines)-1]) == "" {
		lines = lines[:len(lines)-1]
	}

	return lines
}



func SaveToFile(filename string, combinedPatch string) error {
	err := os.WriteFile(filename, []byte(combinedPatch), 0644)
	if err != nil {
		return fmt.Errorf("error writing to file: %v", err)
	}

	return nil
}
