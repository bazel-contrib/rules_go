package main

import (
	"fmt"
	"go/token"
	"strings"

	"golang.org/x/tools/go/analysis"
)

// Change represents a set of edits to be applied to a set of files.
type Change struct {
	AnalysisName string            `json:"analysis_name"`
	FileToEdits  map[string][]Edit `json:"file_to_edits"`
}

// NewChange creates a new Change object.
func NewChange() *Change {
	return &Change{
		FileToEdits: make(map[string][]Edit),
	}
}

// SetAnalysisName sets the name of the analysis that produced the change.
func (c *Change) SetAnalysisName(name string) {
	c.AnalysisName = name
}

// AddEdit adds an edit to the change.
func (c *Change) AddEdit(file string, edit Edit) {
	c.FileToEdits[file] = append(c.FileToEdits[file], edit)
}

// BuildFromDiagnostics builds a Change from a set of diagnostics.
// Unlike Diagnostic, Change is independent of the FileSet given it uses perf-file offsets instead of token.Pos.
// This allows Change to be used in contexts where the FileSet is not available, e.g., it remains applicable after it is saved to disk and loaded back.
// See https://github.com/golang/tools/blob/master/go/analysis/diagnostic.go for details.
func (c *Change) BuildFromDiagnostics(diagnostics []analysis.Diagnostic, fileSet *token.FileSet) error {
	for _, diag := range diagnostics {
		for _, sf := range diag.SuggestedFixes {
			for _, edit := range sf.TextEdits {
				file := fileSet.File(edit.Pos)

				if file == nil {
					return fmt.Errorf("invalid fix: missing file info for pos (%v)", edit.Pos)
				}
				if edit.Pos > edit.End {
					return fmt.Errorf("invalid fix: pos (%v) > end (%v)", edit.Pos, edit.End)
				}
				if eof := token.Pos(file.Base() + file.Size()); edit.End > eof {
					return fmt.Errorf("invalid fix: end (%v) past end of file (%v)", edit.End, eof)
				}
				edit := Edit{Start: file.Offset(edit.Pos), End: file.Offset(edit.End), New: string(edit.NewText)}
				fileRelativePath := file.Name()
				c.AddEdit(fileRelativePath, edit)
			}
		}
	}
	return nil
}

// MergeChanges merges multiple changes into a single change.
func MergeChanges(changes []Change) Change {
	mergedChange := NewChange() // Create a new Change object for the result
	analysisNames := []string{} // no deduplication needed

	for _, change := range changes {
		if change.AnalysisName != "" {
			analysisNames = append(analysisNames, change.AnalysisName)
		}
		for file, edits := range change.FileToEdits {
			// If the file already exists in the merged change, append the edits
			if existingEdits, found := mergedChange.FileToEdits[file]; found {
				// checking the overlapping of edits happens in edit.go during the ApplyEdits function.
				// so we don't need to check it here.
				mergedChange.FileToEdits[file] = append(existingEdits, edits...)
			} else {
				// Otherwise, just set the file and edits
				mergedChange.FileToEdits[file] = edits
			}
		}
	}
	mergedChange.AnalysisName = strings.Join(analysisNames, ",")
	return *mergedChange
}
