#include <iostream>

#include "tests/core/cgo/cc_static_init/lib.h"

int main() {
  const int expected = 42;
  const int actual = *GetValue();
  if (actual == expected) {
    return 0;
  }
  std::cout << "Expected " << expected << ", got " << actual << '\n';
  return 1;
}
