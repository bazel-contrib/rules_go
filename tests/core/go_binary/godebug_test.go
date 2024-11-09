package godebug

import (
	"os/exec"
	"strings"
	"testing"

	"github.com/bazelbuild/rules_go/go/tools/bazel"
)

func TestGodebugDefaults(t *testing.T) {
	tests := map[string]struct {
		binary string
		want   string
	}{
		"debug": {
			binary: "godebug_bin_godebug",
			want:   "http2debug=1",
		},
		"no_debug": {
			binary: "godebug_bin",
			want:   "http2debug=",
		},
	}

	for testName, tc := range tests {
		t.Run(testName, func(t *testing.T) {
			bin, ok := bazel.FindBinary("tests/core/go_binary", tc.binary)
			if !ok {
				t.Fatalf("could not find %v binary", tc.binary)
			}

			out, err := exec.Command(bin).Output()
			if err != nil {
				t.Fatal(err)
			}

			got := strings.TrimSpace(string(out))
			if got != tc.want {
				t.Errorf("got:%v\nwant:%s\n", got, tc.want)
			}
		})
	}
}
