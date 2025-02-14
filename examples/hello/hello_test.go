package main

import (
	"bytes"
	"strings"
	"testing"
)

func TestHello(t *testing.T) {
	out := bytes.NewBuffer(nil)
	sayHello(out)
	got := out.String()
	if !strings.Contains(got, "Hello") {
		t.Errorf("did not print Hello")
	}
}
