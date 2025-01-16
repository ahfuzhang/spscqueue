package spscqueue

import (
	"errors"
	"sync/atomic"
	"syscall"
	"unsafe"

	"gopkg.in/ro-ag/posix.v1"
)

const (
	cacheLineSize = 64       // cache line 的大小
	shmHeaderLen  = 1024 * 4 // 共享内存的头长度，是一个 page 的长度
	minQueueBytes = 1024     // 队列最小的字节数
	itemHeaderLen = 4        // 每个 item 使用 4 字节表示长度
	minLeftLen    = 7        // 尾部剩余的最小字节数
	itemLenMask   = 3        // item的长度的掩码
	shmMode       = 0666     // 共享内存的权限
)

// SpscQueue 共享内存队列的结构
// 共计 1024*4 字节，放置于共享内存的头部
type SpscQueue struct {
	consumer  uint64
	Reserved1 [cacheLineSize - 8]byte // 防止伪共享
	producer  uint64
	Reserved2 [cacheLineSize - 8]byte
	mask      uint64 // 队列长度必须是 2 的 n 次幂
	Reserved3 [shmHeaderLen - cacheLineSize*2 - 8]byte
	// todo: 增加 eventfd
	// todo: 文件锁
}

// NewSpscQueueFromShm 构造队列
// @param shmName 共享内存名称
// @param queueBytes 队列的字节数，必须是 2 的 n 次幂
// @param isCreate 共享内存不存在的时候，是否创建
func NewSpscQueueFromShm(shmName string, queueBytes uint64,
	isCreate bool) (q *SpscQueue, err error) {
	if RoundPowerOfTwo(queueBytes) != queueBytes {
		err = ErrOfBadQueueSize
		return
	}
	var fd int
	var data []byte
	var isFirstTime bool
	fd, err = posix.ShmOpen(shmName, syscall.O_RDWR, shmMode)
	if err != nil {
		if !errors.Is(err, syscall.ENOENT) {
			err = ErrOfOpenShmFail
			return
		}
		if !isCreate {
			err = ErrOfShmNotExist
			return
		}
		fd, err = posix.ShmOpen(shmName, posix.O_CREAT|posix.O_RDWR, shmMode)
		if err != nil {
			err = errors.Join(ErrOfCreateShmFail, err)
			return
		}
		err = posix.Ftruncate(fd, int(queueBytes+shmHeaderLen))
		if err != nil {
			_ = posix.Close(fd)
			err = errors.Join(ErrOfTruncateShm, err)
			return
		}
		isFirstTime = true
	}
	// map
	data, _, err = posix.Mmap(nil, int(queueBytes+shmHeaderLen),
		posix.PROT_READ|posix.PROT_WRITE, posix.MAP_SHARED, fd, 0)
	if err != nil {
		_ = posix.Close(fd)
		err = errors.Join(ErrOfMMapError, err)
		return
	}
	_ = posix.Close(fd)
	//
	q = (*SpscQueue)(unsafe.Pointer(&data[0]))
	if isFirstTime {
		q.consumer = 0
		q.producer = 0
		q.mask = queueBytes - 1
	} else {
		if q.mask+1 != queueBytes {
			err = ErrOfBadMaskOfOpenedShm
			_ = posix.Munmap(data)
			return
		}
	}
	return
}

// NewSpscQueue 在堆上分配 SpscQueue
func NewSpscQueue(queueBytes uint64) (*SpscQueue, error) {
	if RoundPowerOfTwo(queueBytes) != queueBytes {
		return nil, ErrOfBadQueueSize
	}
	totalBytes := shmHeaderLen + queueBytes + minLeftLen
	buf := make([]byte, totalBytes)
	padding := uint64(uintptr(unsafe.Pointer(&buf[0]))) & minLeftLen
	q := (*SpscQueue)(unsafe.Add(unsafe.Pointer(&buf[0]), padding))
	q.consumer = 0
	q.producer = 0
	q.mask = queueBytes - 1
	return q, nil
}

// Alloc 从队尾分配内存
// @param bytes 一个块的大小。一个块不超过 4gb
func (q *SpscQueue) Alloc(needBytes uint32) (out []byte, newTail uint64, err error) {
	// 从队尾申请内存
	if uint64(needBytes) > q.mask/2 {
		err = ErrOfBytesTooLarge
		return
	}
	if needBytes == 0 {
		err = ErrOfBadParam
		return
	}
	for {
		curTail := atomic.LoadUint64(&q.producer)
		curHead := atomic.LoadUint64(&q.consumer) // head 读到版本 2 会怎么样? 会更加准确
		if (curTail+1)&q.mask == curHead {        // ??? 有必要吗
			err = ErrOfNotEnoughSpace // queue is full
			return
		}
		padding := curTail & itemLenMask // 按照 4 字节对齐
		if curTail >= curHead {
			if curTail+minLeftLen+uint64(needBytes) >= q.Len() {
				if q.consumer == 0 {
					err = ErrOfNotEnoughSpace
					return
				}
				if !atomic.CompareAndSwapUint64(&q.producer, curTail, 0) {
					continue
				}
				if curTail+padding+itemHeaderLen <= q.mask {
					// 尾部空间不够的时候，绕到头部
					nextItemPtr := (*uint32)(q.GetPointer(curTail + padding))
					atomic.StoreUint32(nextItemPtr, 0)
				}
				continue
			}
		} else {
			if curTail+minLeftLen+uint64(needBytes)+1 > curHead {
				err = ErrOfNotEnoughSpace
				return
			}
		}
		newTail = curTail + padding + itemHeaderLen + uint64(needBytes)
		nextItemPtr := (*uint32)(q.GetPointer(curTail + padding))
		oldValue := atomic.LoadUint32(nextItemPtr)
		if !atomic.CompareAndSwapUint32(nextItemPtr, oldValue, needBytes) { // todo: 理论上用 atomic_store 就够了
			continue
		}
		out = unsafe.Slice((*byte)(q.GetPointer(curTail+padding+itemHeaderLen)), needBytes)
		return
	}
}

// Len 数据区域的字节数
func (q *SpscQueue) Len() uint64 {
	return q.mask + 1
}

// CommitProduce 提交 Alloc, 尾部位置移动到新位置
func (q *SpscQueue) CommitProduce(newTail uint64) bool {
	curTail := atomic.LoadUint64(&q.producer)
	return atomic.CompareAndSwapUint64(&q.producer, curTail, newTail)
}

// Produce 把 Alloc + CommitProduce 合并到一个方法
func (q *SpscQueue) Produce(src []byte) error {
	dst, newTail, err := q.Alloc(uint32(len(src)))
	if err != nil {
		return err
	}
	copy(dst, src)
	if !q.CommitProduce(newTail) {
		return ErrOfCommitProduceFail
	}
	return nil
}

// GetPointer 返回 queue 的数据区的指针
func (q *SpscQueue) GetPointer(offset uint64) unsafe.Pointer {
	if offset > q.mask {
		panic("offset out of range")
	}
	return unsafe.Add(unsafe.Pointer(q), shmHeaderLen+offset)
}

// IsFull 是否队满
func (q *SpscQueue) IsFull() bool {
	return (atomic.LoadUint64(&q.producer)+1)&q.mask == atomic.LoadUint64(&q.consumer)
}

// IsEmpty 是否队空
func (q *SpscQueue) IsEmpty() bool {
	return atomic.LoadUint64(&q.consumer) == atomic.LoadUint64(&q.producer)
}

// GetOne 读队列头部的 item
func (q *SpscQueue) GetOne() (data []byte, newHead uint64, err error) {
	for {
		curHead := atomic.LoadUint64(&q.consumer)
		curTail := atomic.LoadUint64(&q.producer) // 读到版本 2 无影响
		if curHead == curTail {
			err = ErrOfQueueIsEmpty
			return
		}
		if curHead+minLeftLen >= q.Len() { // 最右侧的 7 字节不会使用
			if curTail > curHead {
				// todo: 一般不可能出现这种情况
				atomic.CompareAndSwapUint64(&q.consumer, curHead, curTail)
			} else {
				atomic.CompareAndSwapUint64(&q.consumer, curHead, 0)
			}
			continue
		}
		padding := curHead & itemLenMask // 按照 4 字节对齐
		nextItemPtr := (*uint32)(q.GetPointer(curHead + padding))
		itemLen := atomic.LoadUint32(nextItemPtr)
		if itemLen == 0 {
			// 0 是特殊标记，表示从这里到尾部都没有数据
			if curTail > curHead {
				//todo: 单元测试中构造这种临界点
				atomic.CompareAndSwapUint64(&q.consumer, curHead, curTail)
			} else {
				atomic.CompareAndSwapUint64(&q.consumer, curHead, 0)
			}
			continue
		}
		newHead = curHead + padding + itemHeaderLen + uint64(itemLen)
		if curTail < curHead {
			if curHead+minLeftLen+uint64(itemLen) > q.Len() {
				panic("impossible error: out of range")
			}
		} else {
			if newHead > curTail {
				panic("impossible error: out of range")
			}
		}
		data = unsafe.Slice((*byte)(q.GetPointer(curHead+padding+itemHeaderLen)), itemLen)
		return
	}
}

// CommitConsume 提交 GetOne()，把 head 修改到新位置
func (q *SpscQueue) CommitConsume(newHead uint64) bool {
	curHead := atomic.LoadUint64(&q.consumer)
	return atomic.CompareAndSwapUint64(&q.consumer, curHead, newHead)
}

// Consume 整合 GetOne() + CommitConsume()
func (q *SpscQueue) Consume(dst []byte) (out []byte, err error) {
	var src []byte
	var newHead uint64
	src, newHead, err = q.GetOne()
	if err != nil {
		return
	}
	out = append(dst, src...)
	if !q.CommitConsume(newHead) {
		err = ErrOfCommitConsumeFail
	}
	return
}

// Unmap 取消共享内存映射
func (q *SpscQueue) Unmap() error {
	data := unsafe.Slice((*byte)(unsafe.Pointer(q)), q.Len()+shmHeaderLen)
	return posix.Munmap(data)
}
