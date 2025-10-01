package foo

import (
	"os"
	"os/exec"
	"testing"
)

func TestBinary(t *testing.T) {
	bar()
	cmd := exec.Command(os.Getenv("RUNFILES_DIR") + "/io_bazel_rules_go/foo/cmd/cmd")
	cmd.Stdout = os.Stdout
	err := cmd.Run()
	if err != nil {
		t.Fatal(err)
	}
}
