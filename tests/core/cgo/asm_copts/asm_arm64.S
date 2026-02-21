#ifndef ASM_COPTS_VALUE
#error "ASM_COPTS_VALUE not defined; copts not passed to assembly"
#endif

/*
define this symbol both with and without underscore.
It seems like Mac OS X wants the underscore, but Linux does not.
*/
.global example_asm_copts_func
.global _example_asm_copts_func
.p2align 2 /* ld: warning: arm64 function not 4-byte aligned */
example_asm_copts_func:
_example_asm_copts_func:
    mov x0, #ASM_COPTS_VALUE
    ret

#if NOT_DEFINED
#error "should not fail"
#endif
