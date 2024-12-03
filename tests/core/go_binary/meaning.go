package main

import(
	"fmt"
	"github.com/bazelbuild/rules_go/tests/core/usesyso"
)

func main() {
	fmt.Println("The meaning of life is:", usesyso.Foo())
}
