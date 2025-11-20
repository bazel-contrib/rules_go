package cgonested

// int bar();
import "C"

func Bar() int {
	return int(C.bar())
}
