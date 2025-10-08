#include "tests/core/cgo/cgo_alwayslink_init/lib.h"

namespace {

struct SideEffect {
  SideEffect() {
    value += 42;
  }
};

SideEffect effect;

}  // namespace
