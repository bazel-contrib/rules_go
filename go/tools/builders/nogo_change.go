package main

import (
	"fmt"
	"go/token"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"unicode"

	"golang.org/x/tools/go/analysis"
)

// DiagnosticEntry represents a diagnostic entry with the corresponding analyzer.
type DiagnosticEntry struct {
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

// The following is about the `Change`, a high-level abstraction of edits.
// Change represents a set of edits to be applied to a set of files.
type Change struct {
	AnalyzerToFileToEdits map[string]map[string][]Edit `json:"analyzer_file_to_edits"`
}

// NewChange creates a new Change object.
func NewChange() *Change {
	return &Change{
		AnalyzerToFileToEdits: make(map[string]map[string][]Edit),
	}
}

// NewChangeFromDiagnostics builds a Change from a set of diagnostics.
// Unlike Diagnostic, Change is independent of the FileSet given it uses perf-file offsets instead of token.Pos.
// This allows Change to be used in contexts where the FileSet is not available, e.g., it remains applicable after it is saved to disk and loaded back.
// See https://github.com/golang/tools/blob/master/go/analysis/diagnostic.go for details.
func NewChangeFromDiagnostics(entries []DiagnosticEntry, fileSet *token.FileSet) (*Change, error) {
	c := NewChange()

	cwd, err := os.Getwd() // workspace root
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
					// In insertion, end could be token.NoPos
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
				c.AddEdit(analyzer, fileRelativePath, edit)
			}
		}
	}

	if len(allErrors) > 0 {
		return c, fmt.Errorf("errors: %v", allErrors)
	}
	return c, nil
}

// AddEdit adds an edit to the change.
func (c *Change) AddEdit(analyzer string, file string, edit Edit) {
	// Check if the analyzer exists in the map
	if _, ok := c.AnalyzerToFileToEdits[analyzer]; !ok {
		// Initialize the map for the analyzer if it doesn't exist
		c.AnalyzerToFileToEdits[analyzer] = make(map[string][]Edit)
	}

	// Append the edit to the list of edits for the specific file under the analyzer
	c.AnalyzerToFileToEdits[analyzer][file] = append(c.AnalyzerToFileToEdits[analyzer][file], edit)
}

// Flatten takes a Change and returns a map of FileToEdits, merging edits from all analyzers.
func Flatten(change Change) map[string][]Edit {
	fileToEdits := make(map[string][]Edit)

	analyzers := make([]string, 0, len(change.AnalyzerToFileToEdits))
	for analyzer := range change.AnalyzerToFileToEdits {
		analyzers = append(analyzers, analyzer)
	}
	sort.Strings(analyzers)
	for _, analyzer := range analyzers {
		// following the order of analyzers, random iteration order over map makes testing flaky
		fileToEditsMap := change.AnalyzerToFileToEdits[analyzer]
		for file, edits := range fileToEditsMap {
			var localEdits []Edit
			if existingEdits, found := fileToEdits[file]; found {
				localEdits = append(existingEdits, edits...)
			} else {
				localEdits = edits
			}

			// Validate the local edits before updating the map
			localEdits, invalidEditIndex := UniqueEdits(localEdits)
			if invalidEditIndex >= 0 {
				// Detected overlapping edits, skip the edits from this analyzer
				// Note: we merge edits from as many analyzers as possible.
				// This allows us to fix as many linter errors as possible. Also, after the initial set
				// of fixing edits are applied to the source code, the next bazel build will run the analyzers again
				// and produce edits that are no longer overlapping.
				continue
			}
			fileToEdits[file] = localEdits
		}
	}

	return fileToEdits
}

// ToPatches converts the edits to patches.
func ToPatches(fileToEdits map[string][]Edit) (map[string]string, error) {
	patches := make(map[string]string)
	for relativeFilePath, edits := range fileToEdits {
		// Skip processing if edits are nil or empty
		if len(edits) == 0 {
			continue
		}

		edits, _ = UniqueEdits(edits)
		contents, err := os.ReadFile(relativeFilePath)
		if err != nil {
			return nil, err
		}

		out, err := ApplyEditsBytes(contents, edits)
		if err != nil {
			return nil, err
		}

		diff := UnifiedDiff{
			// difflib.SplitLines does not handle well the whitespace at the beginning or the end.
			// For example, it would add an extra \n at the end
			// See https://github.com/pmezard/go-difflib/blob/master/difflib/difflib.go#L768
			// trimWhitespaceHeadAndTail is a postprocessing to produce clean patches.
			A: trimWhitespaceHeadAndTail(SplitLines(string(contents))),
			B: trimWhitespaceHeadAndTail(SplitLines(string(out))),
			// standard convention is to use "a" and "b" for the original and new versions of the file
			// discovered by doing `git diff`
			FromFile: fmt.Sprintf("a/%s", relativeFilePath),
			ToFile:   fmt.Sprintf("b/%s", relativeFilePath),
			// git needs lines of context to be able to apply the patch
			// we use 3 lines of context because that's what `git diff` uses
			Context: 3,
		}
		patch, err := GetUnifiedDiffString(diff)
		if err != nil {
			return nil, err
		}
		patches[relativeFilePath] = patch
	}
	return patches, nil
}

func trimWhitespaceHeadAndTail(lines []string) []string {
	if len(lines) == 0 {
		return lines
	}

	// Inner function: returns true if the given string contains any non-whitespace characters.
	hasNonWhitespaceCharacter := func(s string) bool {
		return strings.ContainsFunc(s, func(r rune) bool {
			return !unicode.IsSpace(r)
		})
	}

	// Trim left
	for i := 0; i < len(lines); i++ {
		if hasNonWhitespaceCharacter(lines[i]) {
			lines = lines[i:]
			break
		}
	}

	// Trim right.
	for i := len(lines) - 1; i >= 0; i-- {
		if hasNonWhitespaceCharacter(lines[i]) {
			return lines[:i+1]
		}
	}

	// If we didn't return above, all strings contained only whitespace, so return an empty slice.
	return []string{}
}
