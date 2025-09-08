#include <stdio.h>
#include <cstdint>

int main() {
  // hacky way to dump out long double bytes
  long double i = 42.24l;
  uint8_t* unsafe = reinterpret_cast<uint8_t*>(&i);
  for (int i = 0; i < 16; ++i) {
    printf("%#x ", unsafe[i]);
  }

  printf("\n");

  i = 64.125l;
  for (int i = 0; i < 16; ++i) {
    printf("%#x ", unsafe[i]);
  }

  printf("\n");
}
