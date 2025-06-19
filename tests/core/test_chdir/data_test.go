package test_chdir

import (
	"log"
	"os"
	"testing"
)

const file = "data.txt"

func init() {
	_, err := os.Stat(file)
	if err != nil {
		log.Fatalf("in init(), could not stat %s: %v", file, err)
	}
}

func TestMain(m *testing.M) {
	_, err := os.Stat(file)
	if err != nil {
		log.Fatalf("in TestMain(), could not stat %s: %v", file, err)
	}
	os.Exit(m.Run())
}

func TestLocal(t *testing.T) {
	_, err := os.Stat(file)
	if err != nil {
		t.Errorf("could not stat %s: %v", file, err)
	}
}
