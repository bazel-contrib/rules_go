#include <iostream>
#include <thread>

#include "tests/core/cgo/cc_static_init/lib.h"

int main() {
  // We could be too fast. Give the side_effect some time to complete
  std::this_thread::sleep_for(std::chrono::milliseconds(10));
  const int expected = 42;
  const int actual = *GetValue();
  if (actual == expected) {
    return 0;
  }
  std::cout << "Expected " << expected << ", got " << actual << '\n';
  return 1;
}
