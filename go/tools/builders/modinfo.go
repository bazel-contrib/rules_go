// FROM https://github.com/golang/go/blob/f4e37b8afc01253567fddbdd68ec35632df86b62/src/cmd/go/internal/modload/build.go
package main

import (
	"encoding/hex"
)

// Magic numbers for the buildinfo. These byte sequences are used by the Go
// runtime to locate embedded build information in binaries.
// First added in https://go-review.googlesource.com/c/go/+/123576/4/src/cmd/go/internal/modload/build.go
var (
	infoStart []byte
	infoEnd   []byte
)

func init() {
	var err error
	infoStart, err = hex.DecodeString("3077af0c9274080241e1c107e6d618e6")
	if err != nil {
		panic("invalid infoStart hex string: " + err.Error())
	}
	infoEnd, err = hex.DecodeString("f932433186182072008242104116d8f2")
	if err != nil {
		panic("invalid infoEnd hex string: " + err.Error())
	}
}

// ModInfoData wraps build information with Go's internal magic markers.
// These markers enable the Go runtime to locate and extract build metadata
// via runtime/debug.ReadBuildInfo(). The info string should be formatted
// according to Go's buildinfo text format.
func ModInfoData(info string) []byte {
	return []byte(string(infoStart) + info + string(infoEnd))
}
