#include <sched.h>
#include <spsc_queue.h>
#include <stdio.h>

const char *shmName = "test_1";
const uint64_t queueSize = 1024 * 4;

#define P(fmt, ...)                                                            \
  printf("%s:%d: " fmt "\n", __FILE__, __LINE__, ##__VA_ARGS__)

void test_loc(SpscQueue *q) {
  // 队空
  q->Consumer = 1;
  q->Producer = 1;
  if (!IsEmpty(q)) {
    printf("should be empty\n");
    exit(-1);
    return;
  }
  // 队满
  q->Consumer = 2;
  q->Producer = 1;
  if (!IsFull(q)) {
    printf("should be full\n");
    exit(-1);
    return;
  }
  // 尾部
  q->Consumer = 0;
  q->Producer = Len(q) - 4;
  void *ptr = NULL;
  // const char* data = "it's a test";
  // uint32_t len = strlen(data);
  uint64_t newTail = 0;
  int ret = Alloc(q, 1, &ptr, &newTail);
  if (ret != ErrOfNotEnoughSpace) {
    printf("should be full\n");
    exit(-1);
    return;
  }
}

void test1() {
  int errorNumber = 0;
  SpscQueue *q = NULL;
  int ret = NewSpscQueueFromShm(shmName, queueSize, true, &errorNumber, &q);
  if (ret != NewQueueSuccess) {
    printf("NewSpscQueueFromShm failed, ret=%d, errorNumber=%d\n", ret,
           errorNumber);
    return;
  }
  printf("head=%llu, tail=%llu, mask=%llx\n", q->Consumer, q->Producer,
         q->Mask);
  const char *data = "it's a test";
  uint32_t len = strlen(data);
  ret = Produce(q, data, len);
  if (ret != AllocSuccess) {
    printf("Produce failed, ret=%d\n", ret);
    return;
  }
  printf("head=%llu, tail=%llu, mask=%llx\n", q->Consumer, q->Producer,
         q->Mask);
  char buffer[1024] = {0};
  uint32_t outLen = sizeof(buffer) - 1;
  ret = Consume(q, buffer, &outLen);
  if (ret != GetOneSuccess) {
    printf("Comsume failed, ret=%d\n", ret);
    return;
  }
  printf("head=%llu, tail=%llu, mask=%llx\n", q->Consumer, q->Producer,
         q->Mask);
  if (strncmp(buffer, data, len) != 0 || outLen != len) {
    printf("data not equal\n");
    return;
  }
  test_loc(q);
  printf("success\n");
  Unmap(q);
  q = NULL;
}

const uint64_t n = 10000000;

void produce() {
  int errorNumber = 0;
  SpscQueue *q = NULL;
  int ret = NewSpscQueueFromShm(shmName, queueSize, true, &errorNumber, &q);
  if (ret != NewQueueSuccess) {
    printf("NewSpscQueueFromShm failed, ret=%d, errorNumber=%d\n", ret,
           errorNumber);
    return;
  }
  for (uint64_t i = 0; i < n;) {
    ret = Produce(q, &i, sizeof(i));
    if (ret != AllocSuccess) {
      if (ret == ErrOfNotEnoughSpace) {
        printf("queue full\n");
        sched_yield();
        continue;
      }
      printf("Produce failed, ret=%d\n", ret);
      return;
    }
    i++;
  }
  printf("Produce success\n");
}

void consume() {
  int errorNumber = 0;
  SpscQueue *q = NULL;
  int ret = NewSpscQueueFromShm(shmName, queueSize, true, &errorNumber, &q);
  if (ret != NewQueueSuccess) {
    printf("NewSpscQueueFromShm failed, ret=%d, errorNumber=%d\n", ret,
           errorNumber);
    return;
  }
  uint64_t out;
  uint32_t len;
  for (uint64_t i = 0; i < n;) {
    len = (uint32_t)sizeof(out);
    ret = Consume(q, &out, &len);
    if (ret != GetOneSuccess) {
      if (ret == ErrOfQueueIsEmpty) {
        printf("queue empty\n");
        sched_yield();
        continue;
      }
      printf("Comsume failed, ret=%d\n", ret);
      return;
    }
    if (out != i) {
      printf("data not equal, out=%llu, i=%llu\n", out, i);
      return;
    }
    i++;
  }
  printf("consume success\n");
}

void one_by_one() {
  int errorNumber = 0;
  SpscQueue *q = NULL;
  int ret = NewSpscQueueFromShm(shmName, queueSize, true, &errorNumber, &q);
  if (ret != NewQueueSuccess) {
    P("NewSpscQueueFromShm failed, ret=%d, errorNumber=%d", ret, errorNumber);
    return;
  }
  for (uint64_t i = 0; i < n; i++) {
    // printf("%s:%d  %llu\n", __FUNCTION__, __LINE__, i);
    ret = Produce(q, &i, sizeof(i));
    if (ret != AllocSuccess) {
      if (ret == ErrOfNotEnoughSpace) {
        printf("queue full\n");
        // sched_yield();
        // continue;
      }
      printf("Produce failed, ret=%d\n", ret);
      return;
    }
    //
    uint64_t out;
    uint32_t len = sizeof(out);
    ret = Consume(q, &out, &len);
    if (ret != GetOneSuccess) {
      if (ret == ErrOfQueueIsEmpty) {
        P("queue empty");
        // sched_yield();
        // continue;
      }
      P("Comsume failed, ret=%d", ret);
      return;
    }
    if (out != i) {
      P("data not equal, out=%llu, i=%llu", out, i);
      P("head=%llu, tail=%llu, mask=%llx", q->Consumer, q->Producer, q->Mask);
      return;
    }
  }
  printf("success\n");
}

int main(int argc, char *argv[]) {
  if (argc < 2) {
    test1();
    return 0;
  }
  // shm_unlink(shmName);
  const char *cmd = argv[1];
  if (strcmp(cmd, "produce") == 0) {
    produce();
  }
  if (strcmp(cmd, "consume") == 0) {
    consume();
  }
  if (strcmp(cmd, "one_by_one") == 0) {
    one_by_one();
  }
  if (strcmp(cmd, "shm_unlink") == 0) {
    printf("%s:%d  %d\n", __FUNCTION__, __LINE__, shm_unlink(shmName));
  }
  return 0;
}
