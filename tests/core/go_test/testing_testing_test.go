package testing_testing

import (
	"testing"
)

func TestMain(m *testing.M) {
	if ! testing.Testing() {
		panic("testing.Testing() returned 'false' in a test")
	}
}
