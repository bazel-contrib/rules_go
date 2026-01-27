package tzdata_test

import (
	"testing"
	"time"
)

func TestTzdata(t *testing.T) {
	london, err := time.LoadLocation("Europe/London")
	if err != nil {
		t.Fatal(err)
	}

	datetime := time.Date(2022, 8, 26, 16, 10, 15, 0, london)

	want := "2022-08-26 16:10+0100"
	got := datetime.Format("2006-01-02 15:04-0700")
	if want != got {
		t.Fatal("want %q got %q", want, got)
	}
}
