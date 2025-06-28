package impure_env_test

import (
	"testing"

	"github.com/bazelbuild/rules_go/tests/core/impure_env"
)

func TestEnvironmentWithoutImpureEnv(t *testing.T) {
	got := impure_env.GetTestVar()
	if got != "" {
		t.Errorf("GetTestVar() = %q; want %q", got, "")
	}
}
