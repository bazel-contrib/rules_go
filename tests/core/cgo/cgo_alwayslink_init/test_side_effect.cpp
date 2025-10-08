#include <iostream>

#include "tests/core/cgo/cgo_alwayslink_init/lib.h"

int main() {
  const int expected = 42;
  const int actual = value;
  if (expected == actual) {
    return 0;
  }
  std::cout << "Expected " << expected << ", got " << actual << '\n';
  return 1;
}
