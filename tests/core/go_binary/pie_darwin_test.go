package test

import (
	"debug/macho"
	"fmt"
	"os"
	"testing"

	"github.com/bazelbuild/rules_go/go/tools/bazel"
)

func openMachO(dir, bin string) (*macho.File, error) {
	bin, ok := bazel.FindBinary(dir, bin)
	if !ok {
		return nil, fmt.Errorf("could not find binary: %s", bin)
	}

	f, err := os.Open(bin)
	if err != nil {
		return nil, err
	}

	return macho.NewFile(f)
}

func TestPIE(t *testing.T) {
	m, err := openMachO("tests/core/go_binary", "hello_auto_bin")
	if err != nil {
		t.Fatal(err)
	}

	if m.Flags&macho.FlagPIE == 0 {
		t.Error("MachO binary is not position-independent.")
	}
}

func TestPIESetting(t *testing.T) {
	m, err := openMachO("tests/core/go_binary", "hello_pie_setting_bin")
	if err != nil {
		t.Fatal(err)
	}

	if m.Flags&macho.FlagPIE == 0 {
		t.Error("MachO binary is not position-independent.")
	}
}

func TestPIESettingTest(t *testing.T) {
	m, err := openMachO("tests/core/go_binary", "hello_pie_setting_test_bin")
	if err != nil {
		t.Fatal(err)
	}

	if m.Flags&macho.FlagPIE == 0 {
		t.Error("MachO binary is not position-independent.")
	}
}
