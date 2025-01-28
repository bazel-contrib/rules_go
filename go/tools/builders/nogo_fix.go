package main

import (
	"bytes"
	"errors"
	"fmt"
	"go/token"
	"io"
	"os"
	"path/filepath"
	"sort"

	"github.com/pmezard/go-difflib/difflib"
	"golang.org/x/tools/go/analysis"
)

// diagnosticEntry represents a diagnostic entry with the corresponding analyzer.
type diagnosticEntry struct {
	analysis.Diagnostic
	analyzerName string
}

// A nogoEdit describes the replacement of a portion of a text file.
type nogoEdit struct {
	New   string // the replacement
	Start int    // starting byte offset of the region to replace
	End   int    // (exclusive) ending byte offset of the region to replace
	analyzerName string
}

type fileChange struct {
	fileName string
	changes []nogoEdit
}

func (e nogoEdit) String() string {
	return fmt.Sprintf("{Start:%d,End:%d,New:%q}", e.Start, e.End, e.New)
}

func (e nogoEdit) Equals(other nogoEdit) bool {
	return e.Start == other.Start && e.End == other.End && e.New == other.New
}

// byStartEnd orders a slice of nogoEdits by (start, end) offset.
// This ordering puts insertions (end = start) before deletions
// (end > start) at the same point. We will use a stable sort to preserve
// the order of multiple insertions at the same point.
type byStartEnd []nogoEdit

func (a byStartEnd) Len() int { return len(a) }
func (a byStartEnd) Less(i, j int) bool {
	if a[i].Start != a[j].Start {
		return a[i].Start < a[j].Start
	}
	return a[i].End < a[j].End
}
func (a byStartEnd) Swap(i, j int) { a[i], a[j] = a[j], a[i] }


// applyEdits applies a sequence of nogoEdits to the src byte slice and returns the result.
// Edits are applied in order of start offset; edits with the same start offset are applied in the order they were provided.
// The function assumes that edits are unique, sorted and non-overlapping.
// This is guaranteed by invoking validate() earlier.
func applyEdits(src []byte, edits []nogoEdit) []byte {
	size := len(src)
	// performance only: this computes the size for preallocation to avoid the slice resizing below.
	for _, edit := range edits {
		size += len(edit.New) + edit.Start - edit.End
	}

	out := make([]byte, 0, size)
	lastEnd := 0
	for _, edit := range edits {
		out = append(out, src[lastEnd:edit.Start]...)
		out = append(out, edit.New...)
		lastEnd = edit.End
	}
	out = append(out, src[lastEnd:]...)

	return out
}

// getFixes merges the suggested fixes from all analyzers, returns one fileChange object per file,
// while reporting conflicts as error.
func getFixes(entries []diagnosticEntry, fileSet *token.FileSet) ([]fileChange, error) {
	var allErrors []error
	finalChanges := make(map[string][]nogoEdit)

	for _, entry := range entries {
		// According to the [doc](https://pkg.go.dev/golang.org/x/tools@v0.28.0/go/analysis#Diagnostic),
		// an analyzer may suggest several alternative fixes, but only one should be applied.
		// We will go over all the suggested fixes until the we find one with no conflict
		// with previously selected fixes. No backtracking is used for simplicity and performance. If
		// none of the suggested fixes of a diagnostic can be applied, the diagnostic entry will be skipped
		// with an error message to the user.
		foundApplicableFix := false
		for _, sf := range entry.Diagnostic.SuggestedFixes {
			candidateChanges := make(map[string][]nogoEdit)
			applicable := true
			for _, edit := range sf.TextEdits {
				start, end := edit.Pos, edit.End
				if !end.IsValid() {
					end = start
				}

				file := fileSet.File(start)
				if file == nil {
					// missing file info, most likely due to analyzer bug.
					applicable = false
					break
				}

				fix := nogoEdit{
					Start: file.Offset(start),
					End: file.Offset(end),
					New: string(edit.NewText),
					analyzerName: entry.analyzerName,
				}
				candidateChanges[file.Name()] = append(candidateChanges[file.Name()], fix)
			}
			// validating the edits from current SuggestedFix. All edits from a SuggestedFix must be
			// either accepted or discarded atomically, because a SuggestedFix may move a statement from one place
			// to the other. If we only accept part of the edits, the statement may either appear twice or disappear.
			for fileName, edits := range candidateChanges {
				edits = append(edits, finalChanges[fileName]...)
				var err error
				if candidateChanges[fileName], err = validate(edits); err != nil {
					applicable = false
					break
				}
			}
			if applicable {
				for fileName, edits := range candidateChanges {
					finalChanges[fileName] = edits
				}
				foundApplicableFix = true
				break
			}
			// Move on to the next SuggestedFix of the same Diagnostic if any edit of the current SuggestedFix has issues.
		}
		if !foundApplicableFix {
			allErrors = append(allErrors, fmt.Errorf(
				"ignoring suggested fixes from analyzer %q at %s",
				entry.analyzerName, fileSet.Position(entry.Pos),
			))
		}
	}

	var finalFileChanges []fileChange
	for fileName, edits := range finalChanges {
		finalFileChanges = append(finalFileChanges, fileChange{fileName: fileName, changes: edits})
	}

	if len(allErrors) == 0 {
		return finalFileChanges, nil
	}

	var errMsg bytes.Buffer
	errMsg.WriteString("some suggested fixes are invalid or have conflicts with other fixes:")
	for _, e := range allErrors {
		errMsg.WriteString("\n\t")
		errMsg.WriteString(e.Error())
	}
	errMsg.WriteString("\nplease apply other fixes and rerun the build.")
	return finalFileChanges, errors.New(errMsg.String())
}


// validate whether the list of edits has overlaps or contains invalid ones.
// If there is any issue, an error is returned. Otherwise, the function
// returns a new list of edits that is sorted and unique.
func validate(edits []nogoEdit) ([]nogoEdit, error) {
	if len(edits) == 0 {
		return nil, nil
	}
	validatedEdits := make([]nogoEdit, len(edits))
	// avoid modifying the original slice for safety.
	copy(validatedEdits, edits)
	sort.Stable(byStartEnd(validatedEdits))
	tail := 0
	for i, cur := range validatedEdits {
		if cur.Start > cur.End {
			return nil, fmt.Errorf("invalid suggestion from %q: %s", cur.analyzerName, cur)
		}
		if i > 0 {
			prev := validatedEdits[i-1]
			if prev.Equals(cur) {
				// equivalent ones are safely skipped
				continue
			}

			if prev.End > cur.Start {
				return nil, fmt.Errorf("overlapping suggestions from %q and %q at %s and %s",
					prev.analyzerName, cur.analyzerName, prev, cur)
			}
		}
		validatedEdits[tail] = cur
		tail++
	}
	return validatedEdits[:tail], nil
}


func writePatch(patchFile io.Writer, changes []fileChange) error {
	// sort the changes by file name to make sure the patch is stable.
	sort.Slice(changes, func(i, j int) bool {
		return changes[i].fileName < changes[j].fileName
	})

	for _, c := range changes {
		if len(c.changes) == 0 {
			continue
		}

		contents, err := os.ReadFile(c.fileName)
		if err != nil {
			return fmt.Errorf("failed to read file %s: %v", c.fileName, err)
		}

		// edits are guaranteed to be unique, sorted and non-overlapping
		// see validate() that is called before this function.
		out := applyEdits(contents, c.changes)

		diff := difflib.UnifiedDiff{
			A:        difflib.SplitLines(string(contents)),
			B:        difflib.SplitLines(string(out)),
			FromFile: filepath.Join("a", c.fileName),
			ToFile:   filepath.Join("b", c.fileName),
			Context:  3,
		}

		if err := difflib.WriteUnifiedDiff(patchFile, diff); err != nil {
			return fmt.Errorf("creating patch for %q: %w", c.fileName, err)
		}
	}

	return nil
}
