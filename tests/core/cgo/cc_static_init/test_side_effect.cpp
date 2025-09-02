#include <iostream>

#include "lib.h"

int main() {
  int expected = 42;
  int actual = *GetValue();
  if (actual == expected) {
    return 0;
  }
  std::cout << "Expected " << expected << ", got " << actual << '\n';
  return 1;
}
