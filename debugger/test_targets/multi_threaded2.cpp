#include <pthread.h>
#include <vector>
#include <iostream>
#include <unistd.h>

void* say_hi(void*) {
  while (true) {
    std::cout << "Thread " << gettid() << " reporting in\n";
    sleep(1);
  }
}

int main() {
  std::vector<pthread_t> threads(10);

  for (auto& thread: threads) {
    pthread_create(&thread, nullptr, say_hi, nullptr);
  }

  for (auto& thread: threads) {
    pthread_join(thread, nullptr);
  }
}
