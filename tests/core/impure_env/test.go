package impure_env_test

import (
	"testing"

	"github.com/bazelbuild/rules_go/tests/core/impure_env"
)

func TestEnvironmentWithImpureEnv(t *testing.T) {
	got := impure_env.GetTestVar()
	if got != "test_value" {
		t.Errorf("GetTestVar() = %q; want %q", got, "test_value")
	}
}
