#ifndef _SPSC_QUEUE_H_
#define _SPSC_QUEUE_H_

#include <errno.h>
#include <fcntl.h>
#include <inttypes.h>
#include <stdbool.h>
#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <sys/mman.h>
#include <unistd.h>

#if defined(__linux__)
       #include <sys/mman.h>
       #include <fcntl.h>
#endif


#define cacheLineSize 64   /* cache line 的字节数 */
#define shmHeaderLen 4096  /* 队列的头的长度，正好等于一个 page */
#define minQueueBytes 1024 /* 队列的最少字节数 */

// SpscQueue (Single producer & single consumer) Queue
typedef struct __attribute__((aligned(8))) {
  uint64_t Producer;
  uint8_t Reserved1[cacheLineSize - 8];
  uint64_t Consumer;
  uint8_t Reserved2[cacheLineSize - 8];
  uint64_t Mask;
  uint8_t Reserved3[shmHeaderLen - cacheLineSize * 2 - 8];
} SpscQueue;

enum ErrorCodeOfNewQueue {
  NewQueueSuccess,
  ErrOfBadParam,
  ErrOfShmOpen,
  ErrOfShmNotExists,
  ErrOfShmCreateFail,
  ErrOfShmTuncate,
  ErrOfMMap
};

enum ErrorCodeOfProduce {
  AllocSuccess,
  ErrOfBytesTooLarge,
  ErrOfBadParamForAlloc,
  ErrOfNotEnoughSpace,
  ErrOfCommitFail
};

enum ErrorCodeOfConsume {
  GetOneSuccess,
  ErrOfQueueIsEmpty,
  ErrOfHeadOutOfRange,
  ErrOfOutBufferTooSmall,
  ErrOfCommitConsumeFail
};

// IsPowerOfTwo 是否是 2 的 n 次幂
bool IsPowerOfTwo(uint64_t n);

// RoundPowerOfTwo 取到最接近的 2 的 n 次幂
uint64_t RoundPowerOfTwo(uint64_t n);

// NewSpscQueueFromShm 从共享内存中创建一个 SpscQueue
// @param shmName 共享内存的名称
// @param queueSize 共享内存的大小， 必须使用 RoundPowerOfTwo 来对齐
// @param isCreate 如果共享内存不存在，是否创建
// @param outErrorNumber 输出的错误码
// @param outQueue 输出的 SpscQueue
// @return 0 表示成功，其他表示失败
int NewSpscQueueFromShm(const char *shmName, uint64_t queueSize, bool isCreate,
                        int *outErrorNumber, SpscQueue **outQueue);

// Len 队列的长度
uint64_t Len(const SpscQueue *q);

// IsFull 队列是否满了
bool IsFull(const SpscQueue *q);

// IsEmpty 队列是否为空
bool IsEmpty(const SpscQueue *q);

// Alloc 分配一个长度为 bytes 的内存区域。相当于 生产者分配内存
// @param q 队列
// @param bytes 要分配的字节数
// @param outPtr 输出的指针
// @param outNewTail 输出的新的 tail
// @return 0 表示成功，其他表示失败
int Alloc(SpscQueue *q, uint32_t bytes, void **outPtr, uint64_t *outNewTail);

// CommitProduce 提交分配的内存区域。相当于 生产者提交分配。与 Alloc 成对使用。
// @param q 队列
// @param newTail 新的 tail
// @return true/false 表示成功
bool CommitProduce(SpscQueue *q, uint64_t newTail);

// GetOne 从队列中获取一个数据。相当于 消费者获取数据
// @param q 队列
// @param dst 输出的指针
// @param outLen 输出的长度
// @param outNewHead 输出的新的 head
// @return 0 表示成功，其他表示失败
int GetOne(SpscQueue *q, void **dst, uint32_t *outLen, uint64_t *outNewHead);

// CommitConsume 提交获取的数据。相当于 消费者提交获取。与 GetOne 成对使用。
// @param q 队列
// @param newHead 新的 head
// @return true/false 表示成功
bool CommitConsume(SpscQueue *q, uint64_t newHead);

// Produce 生产数据。相当于 生产者生产数据。整合了 Alloc 和 CommitProduce
// @param q 队列
// @param src 输入的数据的指针
// @param len 数据的长度
// @return 0 表示成功，其他表示失败
int Produce(SpscQueue *q, const void *src, uint32_t len);

// Consume 消费数据。相当于 消费者消费数据。整合了 GetOne 和 CommitConsume
// @param q 队列
// @param dst 输出的数据的指针
// @param len 数据的长度
// @return 0 表示成功，其他表示失败
int Consume(SpscQueue *q, void *dst, uint32_t *len);

// Unmap 解除映射
// @param q 队列
void Unmap(SpscQueue *q);

#endif
