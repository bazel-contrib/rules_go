package main

import(
	"fmt"
)

//go:noescape
func foo() int

func main() {
	fmt.Println("The meaning of life is:", foo())
}
