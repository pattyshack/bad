#include <iostream>

int ref(int& i) {
  int j = i + 1;
  return j;
}

int main(int argc, const char** argv) {
  int i = 1;
  std::cout << i << " " << &i << '\n';
  {
    int i = 2;
    std::cout << i << " " << &i << '\n';
    {
      int i = 3;
      std::cout << i << " " << &i << '\n';

      ref(i);
    }
  }
}
