package test

import (
	"debug/elf"
	"fmt"
	"os"
	"testing"

	"github.com/bazelbuild/rules_go/go/tools/bazel"
)

func openELF(dir, bin string) (*elf.File, error) {
	bin, ok := bazel.FindBinary(dir, bin)
	if !ok {
		return nil, fmt.Errorf("could not find binary: %s", bin)
	}

	f, err := os.Open(bin)
	if err != nil {
		return nil, err
	}

	return elf.NewFile(f)
}

func TestPIESetting(t *testing.T) {
	e, err := openELF("tests/core/go_binary", "hello_pie_setting_bin")
	if err != nil {
		t.Fatal(err)
	}

	// PIE binaries are implemented as shared libraries.
	if e.Type != elf.ET_DYN {
		t.Error("ELF binary is not position-independent.")
	}
}

func TestPIESettingTest(t *testing.T) {
	e, err := openELF("tests/core/go_binary", "hello_pie_setting_test_bin")
	if err != nil {
		t.Fatal(err)
	}

	// PIE binaries are implemented as shared libraries.
	if e.Type != elf.ET_DYN {
		t.Error("ELF binary is not position-independent.")
	}
}

func TestAndroid(t *testing.T) {
	e, err := openELF("tests/core/go_binary", "hello_android_bin")
	if err != nil {
		t.Fatal(err)
	}

	// PIE binaries are implemented as shared libraries.
	if e.Type != elf.ET_DYN {
		t.Error("ELF binary is not position-independent.")
	}
}

func TestPIE(t *testing.T) {
	e, err := openELF("tests/core/go_binary", "hello_pie_bin")
	if err != nil {
		t.Fatal(err)
	}

	// PIE binaries are implemented as shared libraries.
	if e.Type != elf.ET_DYN {
		t.Error("ELF binary is not position-independent.")
	}
}

func TestNoPIE(t *testing.T) {
	e, err := openELF("tests/core/go_binary", "hello_auto_bin")
	if err != nil {
		t.Fatal(err)
	}

	// PIE binaries are implemented as shared libraries.
	if e.Type != elf.ET_EXEC {
		t.Error("ELF binary is not position-dependent.")
	}
}
