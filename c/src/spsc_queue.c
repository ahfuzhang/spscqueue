#include "spsc_queue.h"

#define itemHeaderLen 4
#define minLeftLen 7
#define itemLenMask 3

#define ATOMIC_LOAD(ptr) __atomic_load_n(ptr, __ATOMIC_SEQ_CST)
#define ATOMIC_CAS(ptr, expectedPtr, desired)                                  \
  __atomic_compare_exchange_n(ptr, expectedPtr, desired, 0, __ATOMIC_SEQ_CST,  \
                              __ATOMIC_SEQ_CST)

bool IsPowerOfTwo(uint64_t n) { return n > 0 && (n & (n - 1)) == 0; }

uint64_t RoundPowerOfTwo(uint64_t n) {
  if (n < minQueueBytes) {
    return minQueueBytes;
  }
  if (IsPowerOfTwo(n)) {
    return n;
  }
  int leading_zeros = __builtin_clz(n);
  return 1 << (64 - leading_zeros);
}

int NewSpscQueueFromShm(const char *shmName, uint64_t queueSize, bool isCreate,
                        int *outErrorNumber, SpscQueue **outQueue) {
  if (!IsPowerOfTwo(queueSize)) {
    return ErrOfBadParam;
  }
  int shm_fd = shm_open(shmName, O_RDWR, 0666);
  bool isFirstTime = false;
  if (shm_fd == -1) {
    *outErrorNumber = errno;
    if (*outErrorNumber != ENOENT) {
      return ErrOfShmOpen;
    }
    // 考虑是否创建
    if (!isCreate) {
      return ErrOfShmNotExists;
    }
    isFirstTime = true;
    shm_fd = shm_open(shmName, O_CREAT | O_RDWR, 0666);
    if (shm_fd == -1) {
      *outErrorNumber = errno;
      return ErrOfShmCreateFail;
    }
    int ret = ftruncate(shm_fd, queueSize + shmHeaderLen);
    if (ret == -1) {
      *outErrorNumber = errno;
      close(shm_fd);
      return ErrOfShmTuncate;
    }
  }
  // 将共享内存映射到进程地址空间
  void *shm_ptr = mmap(0, queueSize + shmHeaderLen, PROT_READ | PROT_WRITE,
                       MAP_SHARED, shm_fd, 0);
  if (shm_ptr == MAP_FAILED) {
    *outErrorNumber = errno;
    close(shm_fd);
    return ErrOfMMap;
  }
  close(shm_fd);
  shm_fd = 0;
  SpscQueue *q = (SpscQueue *)shm_ptr;
  if (isFirstTime) {
    q->Producer = 0;
    q->Consumer = 0;
    q->Mask = queueSize - 1;
  }
  *outQueue = q;
  return NewQueueSuccess;
}

uint64_t Len(const SpscQueue *q) { return q->Mask + 1; }

bool IsFull(const SpscQueue *q) {
  uint64_t curHead = ATOMIC_LOAD(&(q->Consumer));
  uint64_t curTail = ATOMIC_LOAD(&(q->Producer));
  return ((curTail + 1) & q->Mask) == curHead;
}

bool IsEmpty(const SpscQueue *q) {
  uint64_t curHead = ATOMIC_LOAD(&(q->Consumer));
  uint64_t curTail = ATOMIC_LOAD(&(q->Producer));
  return curHead == curTail;
}

int Alloc(SpscQueue *q, uint32_t bytes, void **outPtr, uint64_t *outNewTail) {
  uint64_t needBytes = (uint64_t)bytes;
  if (needBytes > q->Mask / 2) {
    return ErrOfBytesTooLarge;
  }
  if (bytes == 0) {
    return ErrOfBadParamForAlloc;
  }
  for (;;) {
    uint64_t curHead = ATOMIC_LOAD(&(q->Consumer));
    uint64_t curTail = ATOMIC_LOAD(&(q->Producer));
    uint64_t padding = curTail & itemLenMask;
    if (curTail >= curHead) {
      if (curTail + minLeftLen > q->Mask) {
        if (curHead == 0) {
          return ErrOfNotEnoughSpace;
        }
        ATOMIC_CAS(&(q->Producer), &curTail, 0);
        continue;
      }
      if (curTail + minLeftLen + needBytes > q->Mask) {
        if (curHead == 0) {
          return ErrOfNotEnoughSpace;
        }
        // 尾部空间不够的时候，绕到头部
        uint32_t *nextItemPtr =
            (uint32_t *)((uint8_t *)q + shmHeaderLen + curTail + padding);
        uint32_t oldValue = ATOMIC_LOAD(nextItemPtr);
        if (!ATOMIC_CAS(nextItemPtr, &oldValue,
                        0)) { // todo: atomic store 也可以
          continue;
        }
        if (!ATOMIC_CAS(&(q->Producer), &curTail, 0)) {
          uint32_t rollback = 0;
          ATOMIC_CAS(nextItemPtr, &rollback, oldValue);
        }
        continue;
      }
    } else {
      if (curTail + minLeftLen + needBytes + 1 > curHead) {
        return ErrOfNotEnoughSpace;
      }
    }
    *outNewTail = curTail + padding + itemHeaderLen + needBytes;
    uint32_t *nextItemPtr =
        (uint32_t *)((uint8_t *)q + shmHeaderLen + curTail + padding);
    uint32_t oldValue = ATOMIC_LOAD(nextItemPtr);
    if (!ATOMIC_CAS(nextItemPtr, &oldValue, needBytes)) {
      continue;
    }
    *outPtr = (uint8_t *)q + shmHeaderLen + curTail + padding + itemHeaderLen;
    return AllocSuccess;
  }
}

bool CommitProduce(SpscQueue *q, uint64_t newTail) {
  uint64_t curTail = ATOMIC_LOAD(&(q->Producer));
  return ATOMIC_CAS(&(q->Producer), &curTail, newTail);
}

int Produce(SpscQueue *q, const void *src, uint32_t len) {
  void *dst = NULL;
  uint64_t newTail = 0;
  int ret = Alloc(q, len, &dst, &newTail);
  if (ret != AllocSuccess) {
    return ret;
  }
  memcpy(dst, src, len);
  if (!CommitProduce(q, newTail)) {
    return ErrOfCommitFail;
  }
  return AllocSuccess;
}

int GetOne(SpscQueue *q, void **dst, uint32_t *outLen, uint64_t *newHead) {
  for (;;) {
    uint64_t curHead = ATOMIC_LOAD(&(q->Consumer));
    uint64_t curTail = ATOMIC_LOAD(&(q->Producer));
    if (curHead == curTail) {
      return ErrOfQueueIsEmpty;
    }
    if (curHead + minLeftLen > q->Mask) {
      if (curTail > curHead) {
        ATOMIC_CAS(&(q->Consumer), &curHead, curTail);
      } else {
        ATOMIC_CAS(&(q->Consumer), &curHead, 0);
      }
      continue;
    }
    uint64_t padding = curHead & itemLenMask;
    uint32_t *nextItemPtr =
        (uint32_t *)((uint8_t *)q + shmHeaderLen + curHead + padding);
    uint32_t itemLen = ATOMIC_LOAD(nextItemPtr);
    if (itemLen == 0) {
      if (curTail > curHead) {
        ATOMIC_CAS(&(q->Consumer), &curHead, curTail);
      } else {
        ATOMIC_CAS(&(q->Consumer), &curHead, 0);
      }
      continue;
    }
    *newHead = curHead + padding + itemHeaderLen + (uint64_t)(itemLen);
    if (curTail < curHead) {
      if (curHead + minLeftLen + (uint64_t)(itemLen) > Len(q)) {
        return ErrOfHeadOutOfRange;
      }
    } else {
      if (*newHead > curTail) {
        return ErrOfHeadOutOfRange;
      }
    }
    *outLen = itemLen;
    *dst = (uint8_t *)q + shmHeaderLen + curHead + padding + itemHeaderLen;
    return GetOneSuccess;
  }
}

bool CommitConsume(SpscQueue *q, uint64_t newHead) {
  uint64_t curHead = ATOMIC_LOAD(&(q->Consumer));
  return ATOMIC_CAS(&(q->Consumer), &curHead, newHead);
}

int Consume(SpscQueue *q, void *dst, uint32_t *len) {
  void *buf = NULL;
  uint32_t itemLen = 0;
  uint64_t newHead = 0;
  int ret = GetOne(q, &buf, &itemLen, &newHead);
  if (ret != GetOneSuccess) {
    return ret;
  }
  if (*len < itemLen) {
    return ErrOfOutBufferTooSmall;
  }
  *len = itemLen;
  memcpy(dst, buf, itemLen);
  if (!CommitConsume(q, newHead)) {
    return ErrOfCommitConsumeFail;
  }
  return GetOneSuccess;
}

void Unmap(SpscQueue *q) { munmap(q, Len(q) + shmHeaderLen); }
