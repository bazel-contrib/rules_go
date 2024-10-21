package syso

import(
	"testing"
	"github.com/bazelbuild/rules_go/tests/core/usesyso"
)

func TestSysoTransitive(t *testing.T) {
	want := int32(42)
	got := usesyso.Foo()
	if want != got {
		t.Errorf("want: %d, got %d", want, got)
	}
}
