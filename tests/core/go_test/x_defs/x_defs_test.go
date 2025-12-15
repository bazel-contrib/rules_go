package x_defs_lib_test

import (
	"os"
	"testing"

	"github.com/bazelbuild/rules_go/go/runfiles"

	"github.com/bazelbuild/rules_go/tests/core/go_test/x_defs/x_defs_lib"
)

var BinGo = "not set"

var DataDep = "not set"

func TestLibGoPath(t *testing.T) {
	libGoPath, err := runfiles.Rlocation(x_defs_lib.LibGo)
	if err != nil {
		t.Fatal(err)
	}
	_, err = os.Stat(libGoPath)
	if err != nil {
		t.Fatal(err)
	}

	dataPath, err := runfiles.Rlocation(x_defs_lib.DataDep)
	if err != nil {
		t.Fatal(err)
	}
	_, err = os.Stat(dataPath)
	if err != nil {
		t.Fatal(err)
	}
}

func TestBinGoPath(t *testing.T) {
	binGoPath, err := runfiles.Rlocation(BinGo)
	if err != nil {
		t.Fatal(err)
	}
	_, err = os.Stat(binGoPath)
	if err != nil {
		t.Fatal(err)
	}

	dataPath, err := runfiles.Rlocation(DataDep)
	if err != nil {
		t.Fatal(err)
	}
	_, err = os.Stat(dataPath)
	if err != nil {
		t.Fatal(err)
	}
}
