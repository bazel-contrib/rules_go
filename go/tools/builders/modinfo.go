// FROM https://github.com/golang/go/blob/f4e37b8afc01253567fddbdd68ec35632df86b62/src/cmd/go/internal/modload/build.go
package main

import (
	"encoding/hex"
)

// Magic numbers for the buildinfo (apparently). First added in
// https://go-review.googlesource.com/c/go/+/123576/4/src/cmd/go/internal/modload/build.go
var (
	infoStart, _ = hex.DecodeString("3077af0c9274080241e1c107e6d618e6")
	infoEnd, _   = hex.DecodeString("f932433186182072008242104116d8f2")
)

func ModInfoData(info string) []byte {
	return []byte(string(infoStart) + info + string(infoEnd))
}
