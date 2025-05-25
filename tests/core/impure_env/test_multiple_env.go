package impure_env_test

import (
	"testing"

	"github.com/bazelbuild/rules_go/tests/core/impure_env"
)

func TestMultipleEnvironmentVars(t *testing.T) {
	got := impure_env.GetMultipleVars()
	want := map[string]string{
		"TEST_VAR1": "value1",
		"TEST_VAR2": "value2",
		"TEST_VAR3": "value3",
	}

	for k, v := range want {
		if got[k] != v {
			t.Errorf("GetMultipleVars()[%q] = %q; want %q", k, got[k], v)
		}
	}
}
