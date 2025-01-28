package main

import (
	"bytes"
	"fmt"
	"go/token"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"

	"golang.org/x/tools/go/analysis"
)

func TestGetFixes_SuccessCases(t *testing.T) {
	fset := token.NewFileSet()
	f := fset.AddFile("file1.go", fset.Base(), 100)
	f.AddLine(0)
	f.AddLine(20)
	f.AddLine(40)
	f.AddLine(60)
	f.AddLine(80)

	f = fset.AddFile("file2.go", fset.Base(), 100)
	f.AddLine(0)
	f.AddLine(20)
	f.AddLine(40)
	f.AddLine(60)
	f.AddLine(80)

	diagnosticEntries := []diagnosticEntry{
		{
			analyzerName: "analyzer1",
			Diagnostic: analysis.Diagnostic{
				SuggestedFixes: []analysis.SuggestedFix{
					{
						// Moving some text in the same file.
						TextEdits: []analysis.TextEdit{
							{Pos: token.Pos(5), End: token.Pos(13), NewText: []byte("new_text")},
							{Pos: token.Pos(55), End: token.Pos(63)},
						},
					},
				},
			},
		},
		{
			analyzerName: "analyzer1",
			Diagnostic: analysis.Diagnostic{
				SuggestedFixes: []analysis.SuggestedFix{
					{
						// Moving some text across files.
						TextEdits: []analysis.TextEdit{
							{Pos: token.Pos(15), End: token.Pos(23), NewText: []byte("new_text")},
							{Pos: token.Pos(155), End: token.Pos(163)},
						},
					},
				},
			},
		},
		{
			analyzerName: "analyzer2",
			Diagnostic: analysis.Diagnostic{
				SuggestedFixes: []analysis.SuggestedFix{
					{
						// Delete some text
						TextEdits: []analysis.TextEdit{
							{Pos: token.Pos(25), End: token.Pos(30)},
						},
					},
				},
			},
		},
		{
			analyzerName: "analyzer2",
			Diagnostic: analysis.Diagnostic{
				SuggestedFixes: []analysis.SuggestedFix{
					{
						// Adding some text.
						TextEdits: []analysis.TextEdit{
							{Pos: token.Pos(115), End: token.Pos(115), NewText: []byte("new_text")},
						},
					},
				},
			},
		},
		{
			analyzerName: "analyzer3",
			Diagnostic: analysis.Diagnostic{
				// multiple suggested fixes, the first one conflict with other fixes.
				SuggestedFixes: []analysis.SuggestedFix{
					{
						// All edits are ignored.
						TextEdits: []analysis.TextEdit{
							{Pos: token.Pos(29), End: token.Pos(39), NewText: []byte("conflicting change")},
							{Pos: token.Pos(65), End: token.Pos(73)},
						},
					},
					{
						// All edits are kept.
						TextEdits: []analysis.TextEdit{
							{Pos: token.Pos(42), End: token.Pos(52), NewText: []byte("good change")},
							{Pos: token.Pos(65), End: token.Pos(73)},
						},
					},
				},
			},
		},
	}

	fileChanges, err := getFixes(diagnosticEntries, fset)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for _, c := range fileChanges {
		sort.Stable(byStartEnd(c.changes))
		var expect []nogoEdit
		switch c.fileName {
		case "file1.go":
			expect = []nogoEdit{
				{Start: 4, End: 12, New: "new_text", analyzerName: "analyzer1"},
				{Start: 14, End: 22, New: "new_text", analyzerName: "analyzer1"},
				{Start: 24, End: 29, analyzerName: "analyzer2"},
				{Start: 41, End: 51, New: "good change", analyzerName: "analyzer3"},
				{Start: 54, End: 62, analyzerName: "analyzer1"},
				{Start: 64, End: 72, analyzerName: "analyzer3"},
			}
		case "file2.go":
			expect = []nogoEdit{
				{Start: 13, End: 13, New: "new_text", analyzerName: "analyzer2"},
				{Start: 53, End: 61, analyzerName: "analyzer1"},
			}
		}
		if !reflect.DeepEqual(c.changes, expect) {
			t.Errorf("unexpected changes for file %s:\n\tgot:\t%v\n\twant:\t%v", c.fileName, c.changes, expect)
		}
	}
}

func TestGetFixes_Conflict(t *testing.T) {
	fset := token.NewFileSet()
	f := fset.AddFile("file1.go", fset.Base(), 100)
	f.AddLine(0)
	f.AddLine(20)
	f.AddLine(40)
	f.AddLine(60)
	f.AddLine(80)

	diagnosticEntries := []diagnosticEntry{
		{
			analyzerName: "analyzer1",
			Diagnostic: analysis.Diagnostic{
				SuggestedFixes: []analysis.SuggestedFix{
					{
						// Moving some text in the same file.
						TextEdits: []analysis.TextEdit{
							{Pos: token.Pos(5), End: token.Pos(13), NewText: []byte("new_text")},
							{Pos: token.Pos(55), End: token.Pos(63)},
						},
					},
				},
			},
		},
		{
			analyzerName: "analyzer2",
			Diagnostic: analysis.Diagnostic{
				SuggestedFixes: []analysis.SuggestedFix{
					{
						// Delete some text.
						TextEdits: []analysis.TextEdit{
							{Pos: token.Pos(55), End: token.Pos(62)},
						},
					},
				},
			},
		},
	}
	expectedError := `ignoring suggested fixes from analyzer "analyzer2"`
	fileChanges, err := getFixes(diagnosticEntries, fset)
	if err == nil || !strings.Contains(err.Error(), expectedError) {
		t.Errorf("expected error: %s\ngot:%v+", expectedError, err)
	}
	expectedChanges := []fileChange{
		{
			fileName: "file1.go",
			changes: []nogoEdit{
				{Start: 4, End: 12, New: "new_text", analyzerName: "analyzer1"},
				{Start: 54, End: 62, analyzerName: "analyzer1"},
			},
		},
	}
	if !reflect.DeepEqual(fileChanges, expectedChanges) {
		t.Errorf("unexpected changes:\n\tgot:\t%v\n\twant:\t%v", fileChanges, expectedChanges)
	}
}

func TestValidate_Success(t *testing.T) {
	edits := []nogoEdit{
		{Start: 20, End: 30, New: "new_text"},
		{Start: 0, End: 10},
		{Start: 20, End: 30, New: "new_text"},
	}
	original := make([]nogoEdit, len(edits))
	copy(original, edits)

	result, err := validate(edits)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !reflect.DeepEqual(edits, original) {
		t.Errorf("validate should not change the input:\n\tgot:\t%v\n\twant:\t%v", edits, original)
	}
	expect := []nogoEdit{
		{Start: 0, End: 10},
		{Start: 20, End: 30, New: "new_text"},
	}
	if !reflect.DeepEqual(result, expect) {
		t.Errorf("unexpected result:\n\tgot:\t%v\n\twant:\t%v", result, expect)
	}
}

func TestValidate_Failure(t *testing.T) {
	tests := []struct{
		name string
		edits []nogoEdit
		expectedErr string
	}{
		{
			name: "conflicts",
			edits: []nogoEdit{
				{Start: 20, End: 30, New: "new_text", analyzerName: "analyzer1"},
				{Start: 25, End: 35, analyzerName: "analyzer2"},
			},
			expectedErr: `overlapping suggestions from "analyzer1" and "analyzer2" at {Start:20,End:30,New:"new_text"} and {Start:25,End:35,New:""}`,
		},
		{
			name: "invalid edits",
			edits: []nogoEdit{
				{Start: 20, End: 10, New: "new_text", analyzerName: "analyzer1"},
			},
			expectedErr: `invalid suggestion from "analyzer1": {Start:20,End:10,New:"new_text"}`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := validate(tt.edits)
			if err == nil {
				t.Fatal("expected error, got nil")
			}
			if err.Error() != tt.expectedErr {
				t.Errorf("unexpected error:\n\tgot:\t%s\n\twant:\t%s", err.Error(), tt.expectedErr)
			}
		})
	}
}

func TestWritePatch(t *testing.T) {
	tmpDir := t.TempDir()

	file1 := tmpDir + "/file1.go"
	err := os.WriteFile(file1, []byte("package main\nfunc Hello() {}\n"), 0644)
	if err != nil {
		t.Fatalf("Failed to create temporary file1.go: %v", err)
	}

	file2 := tmpDir + "/file2.go"

	err = os.WriteFile(file2, []byte("package main\nvar x = 10\n"), 0644)
	if err != nil {
		t.Fatalf("Failed to create temporary file2.go: %v", err)
	}

	tests := []struct {
		name      string
		fileChanges       []fileChange
		expected  string
		expectErr bool
	}{
		{
			name: "valid patch for multiple files",
			fileChanges: []fileChange{
				{fileName: file1, changes: []nogoEdit{{Start: 27, End: 27, New: "\nHello, world!\n"}}}, // Add to function body
				{fileName: file2, changes: []nogoEdit{{Start: 24, End: 24, New: "var y = 20\n"}}},      // Add a new variable
			},
			expected: fmt.Sprintf(`--- %s
+++ %s
@@ -1,3 +1,5 @@
 package main
-func Hello() {}
+func Hello() {
+Hello, world!
+}
 
--- %s
+++ %s
@@ -1,3 +1,4 @@
 package main
 var x = 10
+var y = 20
 
`, filepath.Join("a", file1), filepath.Join("b", file1), filepath.Join("a", file2), filepath.Join("b", file2)),
		},
		{
			name: "file not found",
			fileChanges: []fileChange{
				{fileName: "nonexistent.go", changes: []nogoEdit{{Start: 0, End: 0, New: "new content"}}},
			},
			expectErr: true,
		},
		{
			name:      "no edits",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var patchWriter bytes.Buffer
			err := writePatch(&patchWriter, tt.fileChanges)

			// Verify error expectation
			if (err != nil) != tt.expectErr {
				t.Fatalf("expected error: %v, got: %v", tt.expectErr, err)
			}

			// If no error, verify the patch output
			actual := patchWriter.String()
			if err == nil && actual != tt.expected {
				t.Errorf("expected patch:\n%s\ngot:\n%s", tt.expected, actual)
			}
		})
	}
}
