package cdeps_embed

// #include "cdeps_embed.h"
import "C"

func CallNative() {
	C.native_greeting()
}
