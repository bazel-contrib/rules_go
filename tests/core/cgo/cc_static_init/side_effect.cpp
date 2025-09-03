#include "tests/core/cgo/cc_static_init/lib.h"

__attribute__((constructor))
static void SideEffectExecutedBeforeMain() {
  int* value = GetValue();
  *value += 42;
}
