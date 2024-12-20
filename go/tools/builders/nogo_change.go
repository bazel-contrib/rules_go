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

// A nogoEdit describes the replacement of a portion of a text file.
type nogoEdit struct {
	New   string // the replacement
	Start int    // starting byte offset of the region to replace
	End   int    // (exclusive) ending byte offset of the region to replace
}

// analyzerToEdits represents the mapping of analyzers to their edits for a specific file.
type analyzerToEdits map[string][]nogoEdit // Analyzer as the key, edits as the value

// nogoChange represents a collection of file edits.
// It is a map with file paths as keys and analyzerToEdits as values.
type nogoChange map[string]analyzerToEdits

// newChange creates a new nogoChange object.
func newChange() nogoChange {
	return make(nogoChange)
}

func (e nogoEdit) String() string {
	return fmt.Sprintf("{Start:%d,End:%d,New:%q}", e.Start, e.End, e.New)
}

// sortEdits orders a slice of nogoEdits by (start, end) offset.
// This ordering puts insertions (end = start) before deletions
// (end > start) at the same point, but uses a stable sort to preserve
// the order of multiple insertions at the same point.
func sortEdits(edits []nogoEdit) {
	sort.Stable(byStartEnd(edits))
}

type byStartEnd []nogoEdit

func (a byStartEnd) Len() int { return len(a) }
func (a byStartEnd) Less(i, j int) bool {
	if a[i].Start != a[j].Start {
		return a[i].Start < a[j].Start
	}
	return a[i].End < a[j].End
}
func (a byStartEnd) Swap(i, j int) { a[i], a[j] = a[j], a[i] }


// applyEditsBytes applies a sequence of nogoEdits to the src byte slice and returns the result.
// Edits are applied in order of start offset; edits with the same start offset are applied in the order they were provided.
// applyEditsBytes returns an error if any edit is out of bounds, or if any pair of edits is overlapping.
func applyEditsBytes(src []byte, edits []nogoEdit) ([]byte, error) {
	// assumption: at this point, edits should be unique, sorted and non-overlapping.
	// this is guaranteed in nogo_main.go by invoking flatten() earlier.
	size := len(src)
	// performance only: this computes the size for preallocation to avoid the slice resizing below.
	for _, edit := range edits {
		size += len(edit.New) + edit.Start - edit.End
	}

	// Apply the edits.
	out := make([]byte, 0, size)
	lastEnd := 0
	for _, edit := range edits {
		out = append(out, src[lastEnd:edit.Start]...)
		out = append(out, edit.New...)
		lastEnd = edit.End
	}
	out = append(out, src[lastEnd:]...)

	return out, nil
}

// newChangeFromDiagnostics builds a nogoChange from a set of diagnostics.
// Unlike Diagnostic, nogoChange is independent of the FileSet given it uses perf-file offsets instead of token.Pos.
// This allows nogoChange to be used in contexts where the FileSet is not available, e.g., it remains applicable after it is saved to disk and loaded back.
// See https://github.com/golang/tools/blob/master/go/analysis/diagnostic.go for details.
func newChangeFromDiagnostics(entries []diagnosticEntry, fileSet *token.FileSet) (nogoChange, error) {
	c := newChange()

	cwd, err := os.Getwd()
	if err != nil {
		return c, fmt.Errorf("error getting current working directory: %v", err)
	}

	var allErrors []error

	for _, entry := range entries {
		analyzer := entry.Analyzer.Name
		for _, sf := range entry.Diagnostic.SuggestedFixes {
			for _, edit := range sf.TextEdits {
				// Define start and end positions
				start, end := edit.Pos, edit.End
				if !end.IsValid() {
					end = start
				}

				file := fileSet.File(start)
				if file == nil {
					allErrors = append(allErrors, fmt.Errorf(
						"invalid fix from analyzer %q: missing file info for start=%v",
						analyzer, start,
					))
					continue
				}
				// at this point, given file != nil, it is guaranteed start >= token.Pos(file.Base())

				fileName := file.Name()
				fileRelativePath, err := filepath.Rel(cwd, fileName)
				if err != nil {
					fileRelativePath = fileName // fallback logic
				}

				// Validate start and end positions
				if start > end {
					allErrors = append(allErrors, fmt.Errorf(
						"invalid fix from analyzer %q for file %q: start=%v > end=%v",
						analyzer, fileRelativePath, start, end,
					))
					continue
				}
				if fileEOF := token.Pos(file.Base() + file.Size()); end > fileEOF {
					allErrors = append(allErrors, fmt.Errorf(
						"invalid fix from analyzer %q for file %q: end=%v is past the file's EOF=%v",
						analyzer, fileRelativePath, end, fileEOF,
					))
					continue
				}
				// at this point, it is guaranteed that file.Pos(file.Base()) <= start <= end <= fileEOF.

				// Create the edit
				nEdit := nogoEdit{Start: file.Offset(start), End: file.Offset(end), New: string(edit.NewText)}
				addEdit(c, fileRelativePath, analyzer, nEdit)
			}
		}
	}

	if len(allErrors) > 0 {
		var errMsg bytes.Buffer
		for _, e := range allErrors {
			errMsg.WriteString("\n")
			errMsg.WriteString(e.Error())
		}
		return c, fmt.Errorf("some suggested fixes are invalid:%s", errMsg.String())
	}

	return c, nil
}


// addEdit adds an edit to the nogoChange, organizing by file and analyzer.
func addEdit(c nogoChange, file string, analyzer string, edit nogoEdit) {
	fileEdits, exists := c[file]
	if !exists {
		fileEdits = make(analyzerToEdits)
		c[file] = fileEdits
	}
	fileEdits[analyzer] = append(fileEdits[analyzer], edit)
}

// uniqueSortedEdits returns a list of edits that is sorted and
// contains no duplicate edits. Returns whether there is overlap.
// Deduplication helps in the cases where two analyzers produce duplicate edits.
func uniqueSortedEdits(edits []nogoEdit) ([]nogoEdit, bool) {
	hasOverlap := false
	if len(edits) == 0 {
		return edits, hasOverlap
	}
	equivalent := func(x, y nogoEdit) bool {
		return x.Start == y.Start && x.End == y.End && x.New == y.New
	}
	sortEdits(edits)
	unique := []nogoEdit{edits[0]}
	for i := 1; i < len(edits); i++ {
		prev, cur := edits[i-1], edits[i]
		if equivalent(prev, cur) {
			// equivalent ones are safely skipped
			continue
		}

		unique = append(unique, cur)
		if prev.End > cur.Start {
			// hasOverlap = true means at least one overlap was detected.
			hasOverlap = true
		}
	}
	return unique, hasOverlap
}

type fileToEdits map[string][]nogoEdit // File path as the key, list of nogoEdit as the value

// flatten processes a nogoChange and returns a fileToEdits.
// It also returns an error if any suggested fixes are skipped due to conflicts.
func flatten(change nogoChange) (fileToEdits, error) {
	result := make(fileToEdits)
	var errs []error

	for file, fileEdits := range change {
		// Get a sorted list of analyzers for deterministic processing order
		analyzers := make([]string, 0, len(fileEdits))
		for analyzer := range fileEdits {
			analyzers = append(analyzers, analyzer)
		}
		sort.Strings(analyzers)

		var mergedEdits []nogoEdit
		for _, analyzer := range analyzers {
			edits := fileEdits[analyzer]
			if len(edits) == 0 {
				continue
			}

			// Merge edits into the current list, checking for overlaps
			candidateEdits := append(mergedEdits, edits...)
			candidateEdits, hasOverlap := uniqueSortedEdits(candidateEdits)
			if hasOverlap {
				// Skip edits from this analyzer if merging them would cause overlaps.
				// Collect an error message for the user.
				errMsg := fmt.Errorf(
					"suggested fixes from analyzer %q on file %q are skipped because they conflict with other analyzers",
					analyzer, file,
				)
				errs = append(errs, errMsg)
				continue
			}

			// Update the merged edits
			// At this point, it is guaranteed the edits associated with the file are unique, sorted, and non-overlapping.
			mergedEdits = candidateEdits
		}

		// Store the final merged edits for the file
		result[file] = mergedEdits
	}

	if len(errs) > 0 {
		var errMsg strings.Builder
		errMsg.WriteString("some suggested fixes are skipped due to conflicts in merging fixes from different analyzers for each file:")
		for _, err := range errs {
			errMsg.WriteString("\n")
			errMsg.WriteString(err.Error())
		}
		return result, fmt.Errorf(errMsg.String())
	}

	return result, nil
}

func toCombinedPatch(fte fileToEdits) (string, error) {
	var combinedPatch strings.Builder

	filePaths := make([]string, 0, len(fte))
	for filePath := range fte {
		filePaths = append(filePaths, filePath)
	}
	sort.Strings(filePaths) // Sort file paths alphabetically

	// Iterate over sorted file paths
	for _, filePath := range filePaths {
		edits := fte[filePath]
		if len(edits) == 0 {
			continue
		}

		contents, err := os.ReadFile(filePath)
		if err != nil {
			return "", fmt.Errorf("failed to read file %s: %v", filePath, err)
		}

		// edits are guaranteed to be unique, sorted and non-overlapping
		// see flatten() that is called before this function.
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

