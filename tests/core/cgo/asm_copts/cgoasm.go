//go:build amd64 || arm64

package asm_copts

/*
extern int example_asm_copts_func();
*/
import "C"

func CallAssemblyCopts() int {
	return int(C.example_asm_copts_func())
}
