#include "tests/core/cgo/cc_static_init/lib.h"

namespace {

struct SideEffect {
  SideEffect() {
    int* value = GetValue();
    *value += 42;
  }
};

SideEffect effect;

}  // namespace
