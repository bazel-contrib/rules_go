package importpath_test

import (
	"fmt"
	"testing"

	bar_proto "tests/core/go_proto_library_importpath/bar_go_proto"
	foo_proto "path/to/foo_go"
)

func Test(t *testing.T) {
	bar := &bar_proto.Bar{}
	bar.Value = &foo_proto.Foo{}
	bar.Value.Value = 5

	var expected int64 = 5
	if bar.Value.Value != expected {
		t.Errorf(fmt.Sprintf("Not equal: \n"+
			"expected: %d\n"+
			"actual  : %d", expected, bar.Value.Value))
	}
}
