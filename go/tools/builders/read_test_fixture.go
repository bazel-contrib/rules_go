package main

import (
	_ "embed"
)

//go:embed before.sql
var beforeDoubleQuoteRune string

func doubleQuoteRune() {
	rune := '"'
}

//go:embed after.sql
var afterDoubleQuoteRune string
