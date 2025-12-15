// +build amd64

#include "textflag.h"

TEXT ·foo(SB), NOSPLIT, $0-4
    CALL foo(SB)
    MOVL AX, ret+0(FP)
    RET
