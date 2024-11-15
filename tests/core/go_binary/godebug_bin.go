package main

import (
	"fmt"
	"internal/godebug"
)

func main() {
	http2debug := godebug.New("http2debug")
	fmt.Printf("%v\n", http2debug)
}
