// +build arm64

#include "textflag.h"

TEXT ·foo(SB), NOSPLIT, $0-4
    BL foo(SB)
    MOVW R0, ret+0(FP)
    RET
