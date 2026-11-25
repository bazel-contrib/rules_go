package bzltestutil

import (
	"strings"
	"testing"
)

func TestConvertCoverToLcov(t *testing.T) {
	var tests = []struct {
		name         string
		goCover      string
		expectedLcov string
	}{
		{
			"empty",
			"",
			"",
		},
		{
			"mode only",
			"mode: atomic\n",
			"",
		},
		{
			"single file",
			`mode: count
file.go:0.4,2.10 1 0
`,
			`SF:file.go
DA:0,0
DA:1,0
DA:2,0
LH:0
LF:3
end_of_record
`,
		},
		{
			"blocks without statements",
			`mode: set
file.go:3.24,3.45 1 1
file.go:5.36,5.37 0 0
empty.go:1.30,1.31 0 0
`,
			`SF:file.go
DA:3,1
LH:1
LF:1
end_of_record
SF:empty.go
LH:0
LF:0
end_of_record
`,
		},
		{
			"narrow ranges",
			`mode: atomic
path/to/pkg/file.go:0.1,0.2 5 1
path/to/pkg/file2.go:1.2,1.2 4 2
`,
			`SF:path/to/pkg/file.go
DA:0,1
LH:1
LF:1
end_of_record
SF:path/to/pkg/file2.go
DA:1,2
LH:1
LF:1
end_of_record
`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			in := strings.NewReader(tt.goCover)
			var out strings.Builder
			err := convertCoverToLcov(in, &out)
			if err != nil {
				t.Errorf("convertCoverToLcov returned unexpected error: %+v", err)
			}
			actualLcov := out.String()
			if actualLcov != tt.expectedLcov {
				t.Errorf("covertCoverToLcov returned:\n%q\n, expected:\n%q\n", actualLcov, tt.expectedLcov)
			}
		})
	}
}
