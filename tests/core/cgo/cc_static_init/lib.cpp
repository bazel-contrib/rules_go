#include "tests/core/cgo/cc_static_init/lib.h"

int* GetValue() {
  static int value = 0;
  return &value;
}
