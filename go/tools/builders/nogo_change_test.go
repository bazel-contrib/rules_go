package main

import (
	"go/token"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"golang.org/x/tools/go/analysis"
)

const (
	FileA         = "from"
	FileB         = "to"
	UnifiedPrefix = "--- " + FileA + "\n+++ " + FileB + "\n"
)

// Mock helper to create a mock file in the token.FileSet
func mockFileSet(fileName string, size int) *token.FileSet {
	fset := token.NewFileSet()
	f := fset.AddFile(fileName, fset.Base(), size)
	for i := 0; i < size; i++ {
		f.AddLine(i)
	}
	return fset
}

// Mock analyzers for the test
var (
	analyzer1 = &analysis.Analyzer{Name: "analyzer1"}
	analyzer2 = &analysis.Analyzer{Name: "analyzer2"}
)

// TestAddEdit_MultipleAnalyzers tests addEdit with multiple analyzers and files using reflect.DeepEqual
func TestAddEdit_MultipleAnalyzers(t *testing.T) {
	change := newChange()
	file1 := "file1.go"

	edit1a := nogoEdit{Start: 10, End: 20, New: "code1 from analyzer1"}
	edit1b := nogoEdit{Start: 30, End: 40, New: "code2 from analyzer1"}
	edit2a := nogoEdit{Start: 50, End: 60, New: "code1 from analyzer2"}
	edit2b := nogoEdit{Start: 70, End: 80, New: "code2 from analyzer2"}

	expected := nogoChange{
		file1: analyzerToEdits{
			analyzer1.Name: {edit1a, edit1b},
			analyzer2.Name: {edit2a, edit2b},
		},
	}

	addEdit(change, file1, analyzer1.Name, edit1a)
	addEdit(change, file1, analyzer1.Name, edit1b)
	addEdit(change, file1, analyzer2.Name, edit2a)
	addEdit(change, file1, analyzer2.Name, edit2b)

	if !reflect.DeepEqual(change, expected) {
		t.Fatalf("nogoChange did not match the expected result.\nGot: %+v\nExpected: %+v", change, expected)
	}
}

// Test case for valid, successful cases
func TestNewChangeFromDiagnostics_SuccessCases(t *testing.T) {
	cwd, _ := os.Getwd()
	file1path := filepath.Join(cwd, "file1.go")

	tests := []struct {
		name              string
		fileSet           *token.FileSet
		diagnosticEntries []diagnosticEntry
		expectedEdits     nogoChange
	}{
		{
			name:    "ValidEdits",
			fileSet: mockFileSet(file1path, 100),
			diagnosticEntries: []diagnosticEntry{
				{
					Analyzer: analyzer1,
					Diagnostic: analysis.Diagnostic{
						SuggestedFixes: []analysis.SuggestedFix{
							{
								TextEdits: []analysis.TextEdit{
									{Pos: token.Pos(5), End: token.Pos(10), NewText: []byte("new_text")},
								},
							},
						},
					},
				},
			},
			expectedEdits: nogoChange{
				"file1.go": analyzerToEdits{
					"analyzer1": {
						{New: "new_text", Start: 4, End: 9}, // 0-based offset
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			change, err := newChangeFromDiagnostics(tt.diagnosticEntries, tt.fileSet)
			if err != nil {
				t.Fatalf("expected no error, got: %v", err)
			}
			if !reflect.DeepEqual(change, tt.expectedEdits) {
				t.Fatalf("expected edits: %+v, got: %+v", tt.expectedEdits, change)
			}
		})
	}
}

// Test case for error cases
func TestNewChangeFromDiagnostics_ErrorCases(t *testing.T) {
	cwd, _ := os.Getwd()
	file1path := filepath.Join(cwd, "file1.go")

	tests := []struct {
		name              string
		fileSet           *token.FileSet
		diagnosticEntries []diagnosticEntry
		expectedErr       string
	}{
		{
			name:    "InvalidPosEnd",
			fileSet: mockFileSet(file1path, 100),
			diagnosticEntries: []diagnosticEntry{
				{
					Analyzer: analyzer1,
					Diagnostic: analysis.Diagnostic{
						SuggestedFixes: []analysis.SuggestedFix{
							{
								TextEdits: []analysis.TextEdit{
									{Pos: token.Pos(15), End: token.Pos(10), NewText: []byte("new_text")},
								},
							},
						},
					},
				},
			},
			expectedErr: "some suggested fixes are invalid:\ninvalid fix from analyzer \"analyzer1\" for file \"file1.go\": start=15 > end=10",
		},
		{
			name:    "EndPastEOF",
			fileSet: mockFileSet(file1path, 100),
			diagnosticEntries: []diagnosticEntry{
				{
					Analyzer: analyzer2,
					Diagnostic: analysis.Diagnostic{
						SuggestedFixes: []analysis.SuggestedFix{
							{
								TextEdits: []analysis.TextEdit{
									{Pos: token.Pos(95), End: token.Pos(110), NewText: []byte("new_text")},
								},
							},
						},
					},
				},
			},
			expectedErr: "some suggested fixes are invalid:\ninvalid fix from analyzer \"analyzer2\" for file \"file1.go\": end=110 is past the file's EOF=101",
		},
		{
			name:    "MissingFileInfo",
			fileSet: mockFileSet(file1path, 100),
			diagnosticEntries: []diagnosticEntry{
				{
					Analyzer: analyzer1,
					Diagnostic: analysis.Diagnostic{
						SuggestedFixes: []analysis.SuggestedFix{
							{
								TextEdits: []analysis.TextEdit{
									{Pos: token.Pos(150), End: token.Pos(160), NewText: []byte("new_text")},
								},
							},
						},
					},
				},
			},
			expectedErr: "some suggested fixes are invalid:\ninvalid fix from analyzer \"analyzer1\": missing file info for start=150",
		},
		{
			name:    "MultipleErrors",
			fileSet: mockFileSet(file1path, 100),
			diagnosticEntries: []diagnosticEntry{
				{
					Analyzer: analyzer1,
					Diagnostic: analysis.Diagnostic{
						SuggestedFixes: []analysis.SuggestedFix{
							{
								TextEdits: []analysis.TextEdit{
									{Pos: token.Pos(15), End: token.Pos(10), NewText: []byte("new_text")},  // InvalidPosEnd
									{Pos: token.Pos(95), End: token.Pos(110), NewText: []byte("new_text")}, // EndPastEOF
								},
							},
						},
					},
				},
			},
			expectedErr: `some suggested fixes are invalid:
invalid fix from analyzer "analyzer1" for file "file1.go": start=15 > end=10
invalid fix from analyzer "analyzer1" for file "file1.go": end=110 is past the file's EOF=101`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := newChangeFromDiagnostics(tt.diagnosticEntries, tt.fileSet)
			if err == nil {
				t.Fatalf("expected an error, got none")
			}

			if err.Error() != tt.expectedErr {
				t.Fatalf("expected error:\n%v\ngot:\n%v", tt.expectedErr, err.Error())
			}
		})
	}
}


func TestSortEdits(t *testing.T) {
	tests := []struct {
		name   string
		edits  []nogoEdit
		sorted []nogoEdit
	}{
		{
			name: "already sorted",
			edits: []nogoEdit{
				{New: "a", Start: 0, End: 1},
				{New: "b", Start: 1, End: 2},
				{New: "c", Start: 2, End: 3},
			},
			sorted: []nogoEdit{
				{New: "a", Start: 0, End: 1},
				{New: "b", Start: 1, End: 2},
				{New: "c", Start: 2, End: 3},
			},
		},
		{
			name: "unsorted",
			edits: []nogoEdit{
				{New: "b", Start: 1, End: 2},
				{New: "a", Start: 0, End: 1},
				{New: "c", Start: 2, End: 3},
			},
			sorted: []nogoEdit{
				{New: "a", Start: 0, End: 1},
				{New: "b", Start: 1, End: 2},
				{New: "c", Start: 2, End: 3},
			},
		},
		{
			name: "insert before delete at same position",
			edits: []nogoEdit{
				{New: "", Start: 0, End: 1},       // delete
				{New: "insert", Start: 0, End: 0}, // insert
			},
			sorted: []nogoEdit{
				{New: "insert", Start: 0, End: 0}, // insert comes before delete
				{New: "", Start: 0, End: 1},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sortEdits(tt.edits)
			if !reflect.DeepEqual(tt.edits, tt.sorted) {
				t.Fatalf("expected %v, got %v", tt.sorted, tt.edits)
			}
		})
	}
}


func TestApplyEditsBytes(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		edits    []nogoEdit
		expected string
	}{
		{
			name:     "empty",
			input:    "",
			edits:    []nogoEdit{},
			expected: "",
		},
		{
			name:     "no_diff",
			input:    "gargantuan\n",
			edits:    []nogoEdit{},
			expected: "gargantuan\n",
		},
		{
			name:  "replace_all",
			input: "fruit\n",
			edits: []nogoEdit{
				{Start: 0, End: 5, New: "cheese"},
			},
			expected: "cheese\n",
		},
		{
			name:  "insert_rune",
			input: "gord\n",
			edits: []nogoEdit{
				{Start: 2, End: 2, New: "u"},
			},
			expected: "gourd\n",
		},
		{
			name:  "delete_rune",
			input: "groat\n",
			edits: []nogoEdit{
				{Start: 1, End: 2, New: ""},
			},
			expected: "goat\n",
		},
		{
			name:  "replace_rune",
			input: "loud\n",
			edits: []nogoEdit{
				{Start: 2, End: 3, New: "r"},
			},
			expected: "lord\n",
		},
		{
			name:  "replace_partials",
			input: "blanket\n",
			edits: []nogoEdit{
				{Start: 1, End: 3, New: "u"},
				{Start: 6, End: 7, New: "r"},
			},
			expected: "bunker\n",
		},
		{
			name:  "insert_line",
			input: "1: one\n3: three\n",
			edits: []nogoEdit{
				{Start: 7, End: 7, New: "2: two\n"},
			},
			expected: "1: one\n2: two\n3: three\n",
		},
		{
			name:  "replace_no_newline",
			input: "A",
			edits: []nogoEdit{
				{Start: 0, End: 1, New: "B"},
			},
			expected: "B",
		},
		{
			name:  "delete_empty",
			input: "meow",
			edits: []nogoEdit{
				{Start: 0, End: 4, New: ""},
			},
			expected: "",
		},
		{
			name:  "append_empty",
			input: "",
			edits: []nogoEdit{
				{Start: 0, End: 0, New: "AB\nC"},
			},
			expected: "AB\nC",
		},
		{
			name:  "add_end",
			input: "A",
			edits: []nogoEdit{
				{Start: 1, End: 1, New: "B"},
			},
			expected: "AB",
		},
		{
			name:  "add_newline",
			input: "A",
			edits: []nogoEdit{
				{Start: 1, End: 1, New: "\n"},
			},
			expected: "A\n",
		},
		{
			name:  "delete_front",
			input: "A\nB\nC\nA\nB\nB\nA\n",
			edits: []nogoEdit{
				{Start: 0, End: 4, New: ""},
				{Start: 6, End: 6, New: "B\n"},
				{Start: 10, End: 12, New: ""},
				{Start: 14, End: 14, New: "C\n"},
			},
			expected: "C\nB\nA\nB\nA\nC\n",
		},
		{
			name:  "replace_last_line",
			input: "A\nB\n",
			edits: []nogoEdit{
				{Start: 2, End: 3, New: "C\n"},
			},
			expected: "A\nC\n\n",
		},
		{
			name:  "multiple_replace",
			input: "A\nB\nC\nD\nE\nF\nG\n",
			edits: []nogoEdit{
				{Start: 2, End: 8, New: "H\nI\nJ\n"},
				{Start: 12, End: 14, New: "K\n"},
			},
			expected: "A\nH\nI\nJ\nE\nF\nK\n",
		},
		{
			name:  "extra_newline",
			input: "\nA\n",
			edits: []nogoEdit{
				{Start: 0, End: 1, New: ""},
			},
			expected: "A\n",
		},
		{
			name:  "unified_lines",
			input: "aaa\nccc\n",
			edits: []nogoEdit{
				{Start: 3, End: 3, New: "\nbbb"},
			},
			expected: "aaa\nbbb\nccc\n",
		},
		{
			name:  "complex_replace_with_tab",
			input: `package a

type S struct {
s fmt.Stringer
}
`,
			edits: []nogoEdit{
				{Start: 27, End: 27, New: "\t"},
			},
			expected: `package a

type S struct {
	s fmt.Stringer
}
`,
		},
	}

	for _, tt := range tests {
		tt := tt // capture range variable
		t.Run(tt.name, func(t *testing.T) {
			result, err := applyEditsBytes([]byte(tt.input), tt.edits)
			if err != nil {
				t.Fatalf("applyEditsBytes failed: %v", err)
			}
			if string(result) != tt.expected {
				t.Errorf("applyEditsBytes: got %q, want %q", string(result), tt.expected)
			}
		})
	}
}


// TestUniqueSortedEdits verifies deduplication and overlap detection.
func TestUniqueSortedEdits(t *testing.T) {
	tests := []struct {
		name           string
		edits          []nogoEdit
		want           []nogoEdit
		wantHasOverlap bool
	}{
		{
			name: "overlapping edits",
			edits: []nogoEdit{
				{Start: 0, End: 2, New: "a"},
				{Start: 1, End: 3, New: "b"},
			},
			want:          []nogoEdit{{Start: 0, End: 2, New: "a"}, {Start: 1, End: 3, New: "b"}},
			wantHasOverlap: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, hasOverlap := uniqueSortedEdits(tt.edits)
			if !reflect.DeepEqual(got, tt.want) || hasOverlap != tt.wantHasOverlap {
				t.Fatalf("expected %v, got %v", tt.want, got)
			}
		})
	}
}


func TestFlatten(t *testing.T) {
	tests := []struct {
		name        string
		change      nogoChange
		expected    fileToEdits
		expectedErr string
	}{
		{
			name: "no conflicts",
			change: nogoChange{
				"file1.go": analyzerToEdits{
					"analyzer1": {
						{Start: 0, End: 5, New: "hello"},
					},
					"analyzer2": {
						{Start: 6, End: 10, New: "world"},
					},
				},
			},
			expected: fileToEdits{
				"file1.go": {
					{Start: 0, End: 5, New: "hello"},
					{Start: 6, End: 10, New: "world"},
				},
			},
			expectedErr: "",
		},
		{
			name: "conflicting edits",
			change: nogoChange{
				"file1.go": analyzerToEdits{
					"analyzer1": {
						{Start: 0, End: 5, New: "hello"},
					},
					"analyzer2": {
						{Start: 3, End: 8, New: "world"},
					},
				},
			},
			expected: fileToEdits{
				"file1.go": {
					{Start: 0, End: 5, New: "hello"},
				},
			},
			expectedErr: `some suggested fixes are skipped due to conflicts in merging fixes from different analyzers for each file:
suggested fixes from analyzer "analyzer2" on file "file1.go" are skipped because they conflict with other analyzers`,
		},
		{
			name: "multiple conflicts across multiple files",
			change: nogoChange{
				"file1.go": analyzerToEdits{
					"analyzer1": {
						{Start: 0, End: 5, New: "hello"},
					},
					"analyzer2": {
						{Start: 4, End: 10, New: "world"},
					},
				},
				"file2.go": analyzerToEdits{
					"analyzer3": {
						{Start: 0, End: 3, New: "foo"},
					},
					"analyzer4": {
						{Start: 2, End: 5, New: "bar"},
					},
				},
			},
			expected: fileToEdits{
				"file1.go": {
					{Start: 0, End: 5, New: "hello"},
				},
				"file2.go": {
					{Start: 0, End: 3, New: "foo"},
				},
			},
			expectedErr: `some suggested fixes are skipped due to conflicts in merging fixes from different analyzers for each file:
suggested fixes from analyzer "analyzer2" on file "file1.go" are skipped because they conflict with other analyzers
suggested fixes from analyzer "analyzer4" on file "file2.go" are skipped because they conflict with other analyzers`,
		},
		{
			name: "no edits",
			change: nogoChange{
				"file1.go": analyzerToEdits{
					"analyzer1": {},
				},
			},
			expected:    fileToEdits{"file1.go": nil},
			expectedErr: "",
		},
		{
			name: "all conflicts",
			change: nogoChange{
				"file1.go": analyzerToEdits{
					"analyzer1": {
						{Start: 0, End: 5, New: "hello"},
					},
					"analyzer2": {
						{Start: 1, End: 4, New: "world"},
					},
				},
			},
			expected: fileToEdits{
				"file1.go": {
					{Start: 0, End: 5, New: "hello"},
				},
			},
			expectedErr: `some suggested fixes are skipped due to conflicts in merging fixes from different analyzers for each file:
suggested fixes from analyzer "analyzer2" on file "file1.go" are skipped because they conflict with other analyzers`,
		},
		{
			name: "no overlapping across different files",
			change: nogoChange{
				"file1.go": analyzerToEdits{
					"analyzer1": {
						{Start: 0, End: 5, New: "hello"},
					},
					"analyzer2": {
						{Start: 10, End: 15, New: "world"},
					},
				},
				"file2.go": analyzerToEdits{
					"analyzer3": {
						{Start: 0, End: 3, New: "foo"},
					},
					"analyzer4": {
						{Start: 5, End: 8, New: "bar"},
					},
				},
			},
			expected: fileToEdits{
				"file1.go": {
					{Start: 0, End: 5, New: "hello"},
					{Start: 10, End: 15, New: "world"},
				},
				"file2.go": {
					{Start: 0, End: 3, New: "foo"},
					{Start: 5, End: 8, New: "bar"},
				},
			},
			expectedErr: "",
		},
		{
			name: "conflict in one file multiple analyzers",
			change: nogoChange{
				"file1.go": analyzerToEdits{
					"analyzer1": {
						{Start: 0, End: 5, New: "hello"},
					},
					"analyzer2": {
						{Start: 5, End: 10, New: "world"},
					},
					"analyzer3": {
						{Start: 3, End: 7, New: "foo"},
					},
				},
			},
			expected: fileToEdits{
				"file1.go": {
					{Start: 0, End: 5, New: "hello"},
					{Start: 5, End: 10, New: "world"},
				},
			},
			expectedErr: `some suggested fixes are skipped due to conflicts in merging fixes from different analyzers for each file:
suggested fixes from analyzer "analyzer3" on file "file1.go" are skipped because they conflict with other analyzers`,
		},
	}

	for _, tt := range tests {
		tt := tt // capture range variable
		t.Run(tt.name, func(t *testing.T) {
			result, err := flatten(tt.change)

			// Check for expected errors
			if tt.expectedErr == "" && err != nil {
				t.Fatalf("expected no error, got: %v", err)
			}
			if tt.expectedErr != "" {
				if err == nil {
					t.Fatalf("expected error:\n%v\nbut got none", tt.expectedErr)
				}
				if err.Error() != tt.expectedErr {
					t.Fatalf("expected error:\n%v\ngot:\n%v", tt.expectedErr, err.Error())
				}
			}

			// Check for expected edits
			if !reflect.DeepEqual(result, tt.expected) {
				t.Fatalf("expected edits:\n%+v\ngot:\n%+v", tt.expected, result)
			}
		})
	}
}

func TestToCombinedPatch(t *testing.T) {
	// Helper functions to create and delete temporary files
	createTempFile := func(filename, content string) error {
		return os.WriteFile(filename, []byte(content), 0644)
	}
	deleteFile := func(filename string) {
		os.Remove(filename)
	}

	// Setup: Create temporary files
	err := createTempFile("file1.go", "package main\nfunc Hello() {}\n")
	if err != nil {
		t.Fatalf("Failed to create temporary file1.go: %v", err)
	}
	defer deleteFile("file1.go")

	err = createTempFile("file2.go", "package main\nvar x = 10\n")
	if err != nil {
		t.Fatalf("Failed to create temporary file2.go: %v", err)
	}
	defer deleteFile("file2.go")

	tests := []struct {
		name      string
		fte       fileToEdits
		expected  string
		expectErr bool
	}{
		{
			name: "valid patch for multiple files",
			fte: fileToEdits{
				"file1.go": {{Start: 27, End: 27, New: "\nHello, world!\n"}}, // Add to function body
				"file2.go": {{Start: 24, End: 24, New: "var y = 20\n"}},      // Add a new variable
			},
			expected: `--- a/file1.go
+++ b/file1.go
@@ -1,2 +1,4 @@
 package main
-func Hello() {}
+func Hello() {
+Hello, world!
+}

--- a/file2.go
+++ b/file2.go
@@ -1,2 +1,3 @@
 package main
 var x = 10
+var y = 20
`,
			expectErr: false,
		},
		{
			name: "file not found",
			fte: fileToEdits{
				"nonexistent.go": {{Start: 0, End: 0, New: "new content"}},
			},
			expected:  "",
			expectErr: true,
		},
		{
			name:      "no edits",
			fte:       fileToEdits{},
			expected:  "",
			expectErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			combinedPatch, err := toCombinedPatch(tt.fte)

			// Verify error expectation
			if (err != nil) != tt.expectErr {
				t.Fatalf("expected error: %v, got: %v", tt.expectErr, err)
			}

			// If no error, verify the patch output
			if err == nil && combinedPatch != tt.expected {
				t.Errorf("expected patch:\n%v\ngot:\n%v", tt.expected, combinedPatch)
			}
		})
	}
}

func TestTrimWhitespaceHeadAndTail(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		input []string
		want  []string
	}{
		{
			name:  "Empty slice",
			input: []string{},
			want:  []string{},
		},
		{
			name:  "All empty strings",
			input: []string{"", " ", "\t", "\n"},
			want:  []string{},
		},
		{
			name:  "Leading and trailing empty strings",
			input: []string{"", " ", "hello", "world", " ", ""},
			want:  []string{"hello", "world"},
		},
		{
			name:  "No leading or trailing empty strings",
			input: []string{"hello", "world"},
			want:  []string{"hello", "world"},
		},
		{
			name:  "Single non-empty string",
			input: []string{"hello"},
			want:  []string{"hello"},
		},
	}

	for _, tt := range tests {
		tt := tt

		t.Run(tt.name, func(t *testing.T) {
			got := trimWhitespaceHeadAndTail(tt.input)

			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("trimWhitespaceHeadAndTail() = %v, want %v", got, tt.want)
			}
		})
	}
}

