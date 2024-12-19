package main

import (
	"fmt"
	"go/token"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"sort"
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

// ApplyEdits() and validate() here provide the reference implementation for testing
// applyEditsBytes() from the refactored nogo_change code, now using NogoEdit.
func ApplyEdits(src string, edits []NogoEdit) (string, error) {
	edits, size, err := validate(src, edits)
	if err != nil {
		return "", err
	}

	// Apply edits.
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
		return "", fmt.Errorf("applyEdits: unexpected output size, got %d, want %d", len(out), size)
	}

	return string(out), nil
}

func validate(src string, edits []NogoEdit) ([]NogoEdit, int, error) {
	if !sort.IsSorted(editsSort(edits)) {
		edits = append([]NogoEdit(nil), edits...)
		sortEdits(edits) 
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

// TestAddEdit_MultipleAnalyzers tests addEdit with multiple analyzers and files using reflect.DeepEqual
func TestAddEdit_MultipleAnalyzers(t *testing.T) {
	change := newChange() 
	file1 := "file1.go"

	edit1a := NogoEdit{Start: 10, End: 20, New: "code1 from analyzer1"}
	edit1b := NogoEdit{Start: 30, End: 40, New: "code2 from analyzer1"}
	edit2a := NogoEdit{Start: 50, End: 60, New: "code1 from analyzer2"}
	edit2b := NogoEdit{Start: 70, End: 80, New: "code2 from analyzer2"}

	expected := map[string]NogoFileEdits{
		file1: {
			AnalyzerToEdits: map[string][]NogoEdit{
				analyzer1.Name: {edit1a, edit1b},
				analyzer2.Name: {edit2a, edit2b},
			},
		},
	}

	change.addEdit(file1, analyzer1.Name, edit1a)
	change.addEdit(file1, analyzer1.Name, edit1b)
	change.addEdit(file1, analyzer2.Name, edit2a)
	change.addEdit(file1, analyzer2.Name, edit2b)

	if !reflect.DeepEqual(change.FileToEdits, expected) {
		t.Fatalf("NogoChange.FileToEdits did not match the expected result.\nGot: %+v\nExpected: %+v", change.FileToEdits, expected)
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
		expectedEdits     map[string]NogoFileEdits
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
			expectedEdits: map[string]NogoFileEdits{
				"file1.go": {
					AnalyzerToEdits: map[string][]NogoEdit{
						"analyzer1": {
							{New: "new_text", Start: 4, End: 9}, // 0-based offset
						},
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
			if !reflect.DeepEqual(change.FileToEdits, tt.expectedEdits) {
				t.Fatalf("expected edits: %+v, got: %+v", tt.expectedEdits, change.FileToEdits)
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
			expectedErr: "errors:\ninvalid fix: pos 15 > end 10",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := newChangeFromDiagnostics(tt.diagnosticEntries, tt.fileSet)
			if err == nil {
				t.Fatalf("expected an error, got none")
			}

			if err.Error() != tt.expectedErr {
				t.Fatalf("expected error: %v, got: %v", tt.expectedErr, err)
			}
		})
	}
}

func TestSortEdits(t *testing.T) {
	tests := []struct {
		name   string
		edits  []NogoEdit
		sorted []NogoEdit
	}{
		{
			name: "already sorted",
			edits: []NogoEdit{
				{New: "a", Start: 0, End: 1},
				{New: "b", Start: 1, End: 2},
				{New: "c", Start: 2, End: 3},
			},
			sorted: []NogoEdit{
				{New: "a", Start: 0, End: 1},
				{New: "b", Start: 1, End: 2},
				{New: "c", Start: 2, End: 3},
			},
		},
		{
			name: "unsorted",
			edits: []NogoEdit{
				{New: "b", Start: 1, End: 2},
				{New: "a", Start: 0, End: 1},
				{New: "c", Start: 2, End: 3},
			},
			sorted: []NogoEdit{
				{New: "a", Start: 0, End: 1},
				{New: "b", Start: 1, End: 2},
				{New: "c", Start: 2, End: 3},
			},
		},
		{
			name: "insert before delete at same position",
			edits: []NogoEdit{
				{New: "", Start: 0, End: 1},       // delete
				{New: "insert", Start: 0, End: 0}, // insert
			},
			sorted: []NogoEdit{
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

// TestCases uses NogoEdit now instead of Edit
var TestCases = []struct {
	Name, In, Out, Unified string
	Edits, LineEdits       []NogoEdit // expectation (LineEdits=nil => already line-aligned)
	NoDiff                 bool
}{
	{
		Name: "empty",
		In:   "",
		Out:  "",
	}, {
		Name: "no_diff",
		In:   "gargantuan\n",
		Out:  "gargantuan\n",
	}, {
		Name: "replace_all",
		In:   "fruit\n",
		Out:  "cheese\n",
		Unified: UnifiedPrefix + `
@@ -1 +1 @@
-fruit
+cheese
`[1:],
		Edits:     []NogoEdit{{Start: 0, End: 5, New: "cheese"}},
		LineEdits: []NogoEdit{{Start: 0, End: 6, New: "cheese\n"}},
	}, {
		Name: "insert_rune",
		In:   "gord\n",
		Out:  "gourd\n",
		Unified: UnifiedPrefix + `
@@ -1 +1 @@
-gord
+gourd
`[1:],
		Edits:     []NogoEdit{{Start: 2, End: 2, New: "u"}},
		LineEdits: []NogoEdit{{Start: 0, End: 5, New: "gourd\n"}},
	}, {
		Name: "delete_rune",
		In:   "groat\n",
		Out:  "goat\n",
		Unified: UnifiedPrefix + `
@@ -1 +1 @@
-groat
+goat
`[1:],
		Edits:     []NogoEdit{{Start: 1, End: 2, New: ""}},
		LineEdits: []NogoEdit{{Start: 0, End: 6, New: "goat\n"}},
	}, {
		Name: "replace_rune",
		In:   "loud\n",
		Out:  "lord\n",
		Unified: UnifiedPrefix + `
@@ -1 +1 @@
-loud
+lord
`[1:],
		Edits:     []NogoEdit{{Start: 2, End: 3, New: "r"}},
		LineEdits: []NogoEdit{{Start: 0, End: 5, New: "lord\n"}},
	}, {
		Name: "replace_partials",
		In:   "blanket\n",
		Out:  "bunker\n",
		Unified: UnifiedPrefix + `
@@ -1 +1 @@
-blanket
+bunker
`[1:],
		Edits: []NogoEdit{
			{Start: 1, End: 3, New: "u"},
			{Start: 6, End: 7, New: "r"},
		},
		LineEdits: []NogoEdit{{Start: 0, End: 8, New: "bunker\n"}},
	}, {
		Name: "insert_line",
		In:   "1: one\n3: three\n",
		Out:  "1: one\n2: two\n3: three\n",
		Unified: UnifiedPrefix + `
@@ -1,2 +1,3 @@
 1: one
+2: two
 3: three
`[1:],
		Edits: []NogoEdit{{Start: 7, End: 7, New: "2: two\n"}},
	}, {
		Name: "replace_no_newline",
		In:   "A",
		Out:  "B",
		Unified: UnifiedPrefix + `
@@ -1 +1 @@
-A
\ No newline at end of file
+B
\ No newline at end of file
`[1:],
		Edits: []NogoEdit{{Start: 0, End: 1, New: "B"}},
	}, {
		Name: "delete_empty",
		In:   "meow",
		Out:  "",
		Unified: UnifiedPrefix + `
@@ -1 +0,0 @@
-meow
\ No newline at end of file
`[1:],
		Edits:     []NogoEdit{{Start: 0, End: 4, New: ""}},
		LineEdits: []NogoEdit{{Start: 0, End: 4, New: ""}},
	}, {
		Name: "append_empty",
		In:   "",
		Out:  "AB\nC",
		Unified: UnifiedPrefix + `
@@ -0,0 +1,2 @@
+AB
+C
\ No newline at end of file
`[1:],
		Edits:     []NogoEdit{{Start: 0, End: 0, New: "AB\nC"}},
		LineEdits: []NogoEdit{{Start: 0, End: 0, New: "AB\nC"}},
	},
	{
		Name: "add_end",
		In:   "A",
		Out:  "AB",
		Unified: UnifiedPrefix + `
@@ -1 +1 @@
-A
\ No newline at end of file
+AB
\ No newline at end of file
`[1:],
		Edits:     []NogoEdit{{Start: 1, End: 1, New: "B"}},
		LineEdits: []NogoEdit{{Start: 0, End: 1, New: "AB"}},
	}, {
		Name: "add_empty",
		In:   "",
		Out:  "AB\nC",
		Unified: UnifiedPrefix + `
@@ -0,0 +1,2 @@
+AB
+C
\ No newline at end of file
`[1:],
		Edits:     []NogoEdit{{Start: 0, End: 0, New: "AB\nC"}},
		LineEdits: []NogoEdit{{Start: 0, End: 0, New: "AB\nC"}},
	}, {
		Name: "add_newline",
		In:   "A",
		Out:  "A\n",
		Unified: UnifiedPrefix + `
@@ -1 +1 @@
-A
\ No newline at end of file
+A
`[1:],
		Edits:     []NogoEdit{{Start: 1, End: 1, New: "\n"}},
		LineEdits: []NogoEdit{{Start: 0, End: 1, New: "A\n"}},
	}, {
		Name: "delete_front",
		In:   "A\nB\nC\nA\nB\nB\nA\n",
		Out:  "C\nB\nA\nB\nA\nC\n",
		Unified: UnifiedPrefix + `
@@ -1,7 +1,6 @@
-A
-B
 C
+B
 A
 B
-B
 A
+C
`[1:],
		NoDiff: true,
		Edits: []NogoEdit{
			{Start: 0, End: 4, New: ""},
			{Start: 6, End: 6, New: "B\n"},
			{Start: 10, End: 12, New: ""},
			{Start: 14, End: 14, New: "C\n"},
		},
		LineEdits: []NogoEdit{
			{Start: 0, End: 4, New: ""},
			{Start: 6, End: 6, New: "B\n"},
			{Start: 10, End: 12, New: ""},
			{Start: 14, End: 14, New: "C\n"},
		},
	}, {
		Name: "replace_last_line",
		In:   "A\nB\n",
		Out:  "A\nC\n\n",
		Unified: UnifiedPrefix + `
@@ -1,2 +1,3 @@
 A
-B
+C
+
`[1:],
		Edits:     []NogoEdit{{Start: 2, End: 3, New: "C\n"}},
		LineEdits: []NogoEdit{{Start: 2, End: 4, New: "C\n\n"}},
	},
	{
		Name: "multiple_replace",
		In:   "A\nB\nC\nD\nE\nF\nG\n",
		Out:  "A\nH\nI\nJ\nE\nF\nK\n",
		Unified: UnifiedPrefix + `
@@ -1,7 +1,7 @@
 A
-B
-C
-D
+H
+I
+J
 E
 F
-G
+K
`[1:],
		Edits: []NogoEdit{
			{Start: 2, End: 8, New: "H\nI\nJ\n"},
			{Start: 12, End: 14, New: "K\n"},
		},
		NoDiff: true,
	}, {
		Name:  "extra_newline",
		In:    "\nA\n",
		Out:   "A\n",
		Edits: []NogoEdit{{Start: 0, End: 1, New: ""}},
		Unified: UnifiedPrefix + `@@ -1,2 +1 @@
-
 A
`,
	}, {
		Name:      "unified_lines",
		In:        "aaa\nccc\n",
		Out:       "aaa\nbbb\nccc\n",
		Edits:     []NogoEdit{{Start: 3, End: 3, New: "\nbbb"}},
		LineEdits: []NogoEdit{{Start: 0, End: 4, New: "aaa\nbbb\n"}},
		Unified:   UnifiedPrefix + "@@ -1,2 +1,3 @@\n aaa\n+bbb\n ccc\n",
	}, {
		Name: "60379",
		In: `package a

type S struct {
s fmt.Stringer
}
`,
		Out: `package a

type S struct {
	s fmt.Stringer
}
`,
		Edits:     []NogoEdit{{Start: 27, End: 27, New: "\t"}},
		LineEdits: []NogoEdit{{Start: 27, End: 42, New: "\ts fmt.Stringer\n"}},
		Unified:   UnifiedPrefix + "@@ -1,5 +1,5 @@\n package a\n \n type S struct {\n-s fmt.Stringer\n+\ts fmt.Stringer\n }\n",
	},
}

func TestApply(t *testing.T) {
	t.Parallel()

	for _, tt := range TestCases {
		t.Run(tt.Name, func(t *testing.T) {
			reversedEdits := slices.Clone(tt.Edits)
			slices.Reverse(reversedEdits)
			got, err := ApplyEdits(tt.In, reversedEdits)
			if err != nil {
				t.Fatalf("ApplyEdits failed: %v", err)
			}
			gotBytes, err := applyEditsBytes([]byte(tt.In), tt.Edits)
			if err != nil {
				t.Fatalf("applyEditsBytes failed: %v", err)
			}
			if got != string(gotBytes) {
				t.Fatalf("applyEditsBytes: got %q, want %q", gotBytes, got)
			}
			if got != tt.Out {
				t.Errorf("ApplyEdits: got %q, want %q", got, tt.Out)
			}
			if tt.LineEdits != nil {
				got, err := ApplyEdits(tt.In, tt.LineEdits)
				if err != nil {
					t.Fatalf("ApplyEdits failed: %v", err)
				}
				gotBytes, err := applyEditsBytes([]byte(tt.In), tt.LineEdits)
				if err != nil {
					t.Fatalf("applyEditsBytes failed: %v", err)
				}
				if got != string(gotBytes) {
					t.Fatalf("applyEditsBytes: got %q, want %q", gotBytes, got)
				}
				if got != tt.Out {
					t.Errorf("ApplyEdits: got %q, want %q", got, tt.Out)
				}
			}
		})
	}
}

// TestUniqueSortedEdits verifies deduplication and overlap detection.
func TestUniqueSortedEdits(t *testing.T) {
	tests := []struct {
		name           string
		edits          []NogoEdit
		want           []NogoEdit
		wantHasOverlap bool
	}{
		{
			name: "overlapping edits",
			edits: []NogoEdit{
				{Start: 0, End: 2, New: "a"},
				{Start: 1, End: 3, New: "b"},
			},
			want:          []NogoEdit{{Start: 0, End: 2, New: "a"}, {Start: 1, End: 3, New: "b"}},
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
		name   string
		change NogoChange
		want   map[string][]NogoEdit
	}{
		{
			name: "multiple analyzers with non-overlapping edits",
			change: NogoChange{
				FileToEdits: map[string]NogoFileEdits{
					"file1.go": {
						AnalyzerToEdits: map[string][]NogoEdit{
							"analyzer1": {{Start: 0, End: 1, New: "a"}},
							"analyzer2": {{Start: 2, End: 3, New: "b"}},
						},
					},
				},
			},
			want: map[string][]NogoEdit{
				"file1.go": {
					{Start: 0, End: 1, New: "a"},
					{Start: 2, End: 3, New: "b"},
				},
			},
		},
		{
			name: "multiple analyzers with overlapping edits",
			change: NogoChange{
				FileToEdits: map[string]NogoFileEdits{
					"file1.go": {
						AnalyzerToEdits: map[string][]NogoEdit{
							"analyzer1": {{Start: 0, End: 2, New: "a"}},
							"analyzer2": {{Start: 1, End: 3, New: "b"}},
						},
					},
				},
			},
			want: map[string][]NogoEdit{
				"file1.go": {
					{Start: 0, End: 2, New: "a"},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := flatten(tt.change)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("flatten() = %v, want %v", got, tt.want)
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
		name        string
		fileToEdits map[string][]NogoEdit
		expected    string
		expectErr   bool
	}{
		{
			name: "valid patch for multiple files",
			fileToEdits: map[string][]NogoEdit{
				"file1.go": {{Start: 27, End: 27, New: "\nHello, world!\n"}}, // Add to function body
				"file2.go": {{Start: 24, End: 24, New: "var y = 20\n"}},       // Add a new variable
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
			fileToEdits: map[string][]NogoEdit{
				"nonexistent.go": {{Start: 0, End: 0, New: "new content"}},
			},
			expected:  "",
			expectErr: true,
		},
		{
			name:      "no edits",
			fileToEdits: map[string][]NogoEdit{},
			expected:  "",
			expectErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			combinedPatch, err := toCombinedPatch(tt.fileToEdits)

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

