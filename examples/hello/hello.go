package main

import (
	"fmt"
	"io"
	"os"
)

func main() {
	sayHello(os.Stdout)
}

func sayHello(w io.Writer) {
	fmt.Fprintf(w, "Hello Bazel! 💚\n")
}
