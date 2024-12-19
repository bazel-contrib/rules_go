package main

import (
	"bytes"
	"fmt"
	"go/token"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/pmezard/go-difflib/difflib"
	"golang.org/x/tools/go/analysis"
)

// diagnosticEntry represents a diagnostic entry with the corresponding analyzer.
type diagnosticEntry struct {
	analysis.Diagnostic
	*analysis.Analyzer
}

// This file contains two main entities: NogoEdit and NogoChange, which correspond to the low-level
// and high-level abstractions. See them below.

// The following is about the `NogoEdit`, a low-level abstraction of edits.
// A NogoEdit describes the replacement of a portion of a text file.
type NogoEdit struct {
	New   string // the replacement
	Start int    // starting byte offset of the region to replace
	End   int    // (exclusive) ending byte offset of the region to replace
}

// NogoFileEdits represents the mapping of analyzers to their edits for a specific file.
type NogoFileEdits struct {
	AnalyzerToEdits map[string][]NogoEdit // Analyzer as the key, edits as the value
}

// NogoChange represents a collection of file edits.
type NogoChange struct {
	FileToEdits map[string]NogoFileEdits // File path as the key, analyzer-to-edits mapping as the value
}

// newChange creates a new NogoChange object.
func newChange() *NogoChange {
	return &NogoChange{
		FileToEdits: make(map[string]NogoFileEdits),
	}
}

func (e NogoEdit) String() string {
	return fmt.Sprintf("{Start:%d,End:%d,New:%q}", e.Start, e.End, e.New)
}

// sortEdits orders a slice of NogoEdits by (start, end) offset.
// This ordering puts insertions (end = start) before deletions
// (end > start) at the same point, but uses a stable sort to preserve
// the order of multiple insertions at the same point.
// (applyEditsBytes detects multiple deletions at the same point as an error.)
func sortEdits(edits []NogoEdit) {
	sort.Stable(editsSort(edits))
}

type editsSort []NogoEdit

func (a editsSort) Len() int { return len(a) }
func (a editsSort) Less(i, j int) bool {
	if cmp := a[i].Start - a[j].Start; cmp != 0 {
		return cmp < 0
	}
	return a[i].End < a[j].End
}
func (a editsSort) Swap(i, j int) { a[i], a[j] = a[j], a[i] }


// validateBytes checks that edits are consistent with the src byte slice,
// and returns the size of the patched output. It may return a different slice if edits are sorted.
func validateBytes(src []byte, edits []NogoEdit) ([]NogoEdit, int, error) {
	if !sort.IsSorted(editsSort(edits)) {
		edits = append([]NogoEdit(nil), edits...)
		sortEdits(edits)
	}

	size := len(src)
	lastEnd := 0
	for _, edit := range edits {
		if !(0 <= edit.Start && edit.Start <= edit.End && edit.End <= len(src)) {
			return nil, 0, fmt.Errorf("the fix has an out-of-bounds edit with start=%d, end=%d", edit.Start, edit.End)
		}
		if edit.Start < lastEnd {
			return nil, 0, fmt.Errorf("the fix has an edit with start=%d, which overlaps with a previous edit with end=%d", edit.Start, lastEnd)
		}
		size += len(edit.New) + edit.Start - edit.End
		lastEnd = edit.End
	}

	return edits, size, nil
}

// applyEditsBytes applies a sequence of NogoEdits to the src byte slice and returns the result.
// Edits are applied in order of start offset; edits with the same start offset are applied in the order they were provided.
// applyEditsBytes returns an error if any edit is out of bounds, or if any pair of edits is overlapping.
func applyEditsBytes(src []byte, edits []NogoEdit) ([]byte, error) {
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
		return nil, fmt.Errorf("applyEditsBytes: unexpected output size, got %d, want %d", len(out), size)
	}

	return out, nil
}


// newChangeFromDiagnostics builds a NogoChange from a set of diagnostics.
// Unlike Diagnostic, NogoChange is independent of the FileSet given it uses perf-file offsets instead of token.Pos.
// This allows NogoChange to be used in contexts where the FileSet is not available, e.g., it remains applicable after it is saved to disk and loaded back.
// See https://github.com/golang/tools/blob/master/go/analysis/diagnostic.go for details.
func newChangeFromDiagnostics(entries []diagnosticEntry, fileSet *token.FileSet) (*NogoChange, error) {
	c := newChange()

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

				nogoEdit := NogoEdit{Start: file.Offset(start), End: file.Offset(end), New: string(edit.NewText)}
				fileRelativePath, err := filepath.Rel(cwd, file.Name())
				if err != nil {
					fileRelativePath = file.Name() // fallback logic
				}
				c.addEdit(fileRelativePath, analyzer, nogoEdit)
			}
		}
	}

	if len(allErrors) > 0 {
		var errMsg bytes.Buffer
		sep := ""
		for _, err := range allErrors {
			errMsg.WriteString(sep)
			sep = "\n"
			errMsg.WriteString(err.Error())
		}
		return c, fmt.Errorf("errors:\n%s", errMsg.String())
	}

	return c, nil
}

// addEdit adds an edit to the NogoChange, organizing by file and analyzer.
func (c *NogoChange) addEdit(file string, analyzer string, edit NogoEdit) {
	// Ensure the NogoFileEdits structure exists for the file
	fileEdits, exists := c.FileToEdits[file]
	if !exists {
		fileEdits = NogoFileEdits{
			AnalyzerToEdits: make(map[string][]NogoEdit),
		}
		c.FileToEdits[file] = fileEdits
	}

	// Append the edit to the list of edits for the analyzer
	fileEdits.AnalyzerToEdits[analyzer] = append(fileEdits.AnalyzerToEdits[analyzer], edit)
}

// uniqueSortedEdits returns a list of edits that is sorted and
// contains no duplicate edits. Returns whether there is overlap.
// Deduplication helps in the cases where two analyzers produce duplicate edits.
func uniqueSortedEdits(edits []NogoEdit) ([]NogoEdit, bool) {
	hasOverlap := false
	if len(edits) == 0 {
		return edits, hasOverlap
	}
	equivalent := func(x, y NogoEdit) bool {
		return x.Start == y.Start && x.End == y.End && x.New == y.New
	}
	sortEdits(edits)
	unique := []NogoEdit{edits[0]}
	for i := 1; i < len(edits); i++ {
		prev, cur := edits[i-1], edits[i]
		if !equivalent(prev, cur) { // equivalent ones are safely skipped
			unique = append(unique, cur)
			if prev.End > cur.Start {
				// hasOverlap = true means at least one overlap was detected.
				hasOverlap = true
			}
		}
	}
	return unique, hasOverlap
}

// flatten merges all edits for a file from different analyzers into a single map of file-to-edits.
// Edits from each analyzer are processed in a deterministic order, and overlapping edits are skipped.
func flatten(change NogoChange) map[string][]NogoEdit {
	fileToEdits := make(map[string][]NogoEdit)

	for file, fileEdits := range change.FileToEdits {
		// Get a sorted list of analyzers for deterministic processing order
		analyzers := make([]string, 0, len(fileEdits.AnalyzerToEdits))
		for analyzer := range fileEdits.AnalyzerToEdits {
			analyzers = append(analyzers, analyzer)
		}
		sort.Strings(analyzers)

		mergedEdits := make([]NogoEdit, 0)

		for _, analyzer := range analyzers {
			edits := fileEdits.AnalyzerToEdits[analyzer]
			if len(edits) == 0 {
				continue
			}

			// Merge edits into the current list, checking for overlaps
			candidateEdits := append(mergedEdits, edits...)
			candidateEdits, hasOverlap := uniqueSortedEdits(candidateEdits)
			if hasOverlap {
				// Skip edits from this analyzer if merging them would cause overlaps.
				// Apply the non-overlapping edits first. After that, a rerun of bazel build will
				// allows these skipped edits to be applied separately.
				// Note the resolution happens to each file independently.
				// Also for clarity, we would accept all or none of an analyzer.
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

// toCombinedPatch converts all edits to a single consolidated patch.
func toCombinedPatch(fileToEdits map[string][]NogoEdit) (string, error) {
	var combinedPatch strings.Builder

	filePaths := make([]string, 0, len(fileToEdits))
	for filePath := range fileToEdits {
		filePaths = append(filePaths, filePath)
	}
	sort.Strings(filePaths) // Sort file paths alphabetically

	// Iterate over sorted file paths
	for _, filePath := range filePaths {
		// edits are unique and sorted, as ensured by the flatten() method that is invoked earlier.
		// for performance reason, let us skip uniqueSortedEdits() call here,
		// although in general a library API shall not assume other calls have been made.
		edits := fileToEdits[filePath]
		if len(edits) == 0 {
			continue
		}

		contents, err := os.ReadFile(filePath)
		if err != nil {
			return "", fmt.Errorf("failed to read file %s: %v", filePath, err)
		}

		out, err := applyEditsBytes(contents, edits)
		if err != nil {
			return "", fmt.Errorf("failed to apply edits for file %s: %v", filePath, err)
		}

		diff := difflib.UnifiedDiff{
			A:        trimWhitespaceHeadAndTail(difflib.SplitLines(string(contents))),
			B:        trimWhitespaceHeadAndTail(difflib.SplitLines(string(out))),
			FromFile: fmt.Sprintf("a/%s", filePath),
			ToFile:   fmt.Sprintf("b/%s", filePath),
			Context:  3,
		}

		patch, err := difflib.GetUnifiedDiffString(diff)
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

