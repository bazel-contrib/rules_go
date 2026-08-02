package testmain_signature

import "testing"

var testMainRan bool

// TestMain is the package entry point only when its parameter is *testing.M
// (covered by only_testmain_test). With a *testing.T parameter it is an
// ordinary test that merely happens to be named TestMain, so this package has
// no entry point: it must still build, and this function must be run as a
// test rather than be called with an *testing.M.
func TestMain(t *testing.T) {
	testMainRan = true
}

// Declared after TestMain on purpose: tests run in declaration order, so by
// the time this one runs, TestMain has run if it was registered as a test.
func TestTestMainRanAsAnOrdinaryTest(t *testing.T) {
	if !testMainRan {
		t.Error("TestMain(*testing.T) was not registered as an ordinary test")
	}
}
