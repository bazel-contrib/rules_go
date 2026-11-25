package main

import (
	"os/exec"
	"strings"
	"testing"

	"github.com/bazelbuild/rules_go/go/tools/bazel"
)

func TestStampOverride(t *testing.T) {
	bin, ok := bazel.FindBinary("tests/core/go_binary", "stamp_override_bin")
	if !ok {
		t.Fatal("could not find stamp_override_bin")
	}

	out, err := exec.Command(bin).Output()
	if err != nil {
		t.Fatal(err)
	}

	got := strings.TrimSpace(string(out))
	want := "bin_version"
	if got != want {
		t.Errorf("got:\n%s\nwant: %s", got, want)
	}
}
