#include "lib.h"

int* GetValue() {
  static int value = 0;
  return &value;
}
