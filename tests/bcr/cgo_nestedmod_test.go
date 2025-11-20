package cgonested

import "testing"

func TestBar(t *testing.T) {
	want := 278
	if got := Bar(); got != want {
		t.Errorf("got %d ; want %d", got, want)
	}
}
