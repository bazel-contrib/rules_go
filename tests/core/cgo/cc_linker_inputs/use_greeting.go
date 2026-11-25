package main

// extern int greeting();
import "C"

func main() { C.greeting() }
