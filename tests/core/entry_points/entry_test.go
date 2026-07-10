package entry_lib

import "testing"

func TestMessage(t *testing.T) {
	if Message() != "Hello from entry_lib" {
		t.Errorf("Unexpected message")
	}
}
