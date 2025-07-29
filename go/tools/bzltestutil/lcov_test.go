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
file.go:0.4,2.10 0 0
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
