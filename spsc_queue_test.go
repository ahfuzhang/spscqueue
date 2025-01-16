package spscqueue

import (
	"encoding/binary"
	"errors"
	"log"
	"runtime"
	"sync"
	"testing"
	"unsafe"

	"gopkg.in/ro-ag/posix.v1"
)

const (
	shmNameOfQueue = "my-test-queue"
	queueBytes     = 1024 * 4
)

func Test_NewQueue_happy_path(t *testing.T) {
	posix.ShmUnlink(shmNameOfQueue)
	q, err := NewSpscQueueFromShm(
		shmNameOfQueue,
		queueBytes,
		true,
	)
	h := unsafe.Offsetof(q.consumer)
	tail := unsafe.Offsetof(q.producer)
	m := unsafe.Offsetof(q.mask)
	if uint64(tail)-uint64(h) != cacheLineSize {
		t.Error("consumer and producer should be aligned on cache line boundary")
		return
	}
	if uint64(m)-uint64(tail) != cacheLineSize {
		t.Error("mask should be aligned on cache line boundary")
		return
	}
	if int64(unsafe.Sizeof(SpscQueue{})) != shmHeaderLen {
		t.Error("RingBuffer is not 4096")
		return
	}
	if err != nil {
		t.Error(err)
		return
	}
	defer q.Unmap()
	//
	var (
		data = []byte("it's a test")
	)
	err = q.Produce(data)
	if err != nil {
		t.Error(err)
		return
	}
	var out []byte
	out, err = q.Consume(nil)
	if err != nil {
		t.Error(err)
		return
	}
	if string(out) != string(data) {
		t.Error("data not match")
		return
	}
	if q.IsFull() {
		t.Error("should be empty")
		return
	}
	if !q.IsEmpty() {
		t.Error("should be empty")
		return
	}
	//
	err = q.Produce(make([]byte, queueBytes/2))
	if err == nil {
		t.Error("should be error")
		return
	}
	_, err = q.Consume(nil)
	if err == nil {
		t.Error("should be error")
		return
	}
}

func Test_NewSpscQueueFromShm(t *testing.T) {
	posix.ShmUnlink(shmNameOfQueue)
	type args struct {
		shmName    string
		queueBytes uint64
		isCreate   bool
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		{
			args: args{
				shmName:    shmNameOfQueue,
				queueBytes: queueBytes + 1,
				isCreate:   false,
			},
			wantErr: true,
		},
		{
			args: args{
				shmName:    shmNameOfQueue,
				queueBytes: queueBytes,
				isCreate:   false,
			},
			wantErr: true,
		},
		{
			args: args{
				shmName:    shmNameOfQueue,
				queueBytes: queueBytes,
				isCreate:   true,
			},
			wantErr: false,
		},
		{
			args: args{
				shmName:    shmNameOfQueue,
				queueBytes: queueBytes / 2,
				isCreate:   false,
			},
			wantErr: true,
		},
		{
			args: args{
				shmName:    shmNameOfQueue,
				queueBytes: queueBytes,
				isCreate:   false,
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewSpscQueueFromShm(tt.args.shmName, tt.args.queueBytes, tt.args.isCreate)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewSpscQueueFromShm() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
		})
	}
}

func TestRingBufferQueue_Alloc(t *testing.T) {
	posix.ShmUnlink(shmNameOfQueue)
	q, err := NewSpscQueueFromShm(
		shmNameOfQueue,
		queueBytes,
		true,
	)
	if err != nil {
		t.Error(err)
		return
	}
	defer q.Unmap()
	// 超过空间的一半会失败
	_, _, err = q.Alloc(queueBytes / 2)
	if !errors.Is(err, ErrOfBytesTooLarge) {
		t.Error("should be: bytes too large")
		return
	}
	_, _, err = q.Alloc(queueBytes/2 - 1)
	if err != nil {
		t.Error(err)
		return
	}
	// 参数为 0，错误
	_, _, err = q.Alloc(0)
	if !errors.Is(err, ErrOfBadParam) {
		t.Error("should be: ErrOfBadParam")
		return
	}
	// 队满的情况
	q.consumer = 1
	_, _, err = q.Alloc(1)
	if !errors.Is(err, ErrOfNotEnoughSpace) {
		t.Error("should be: ErrOfNotEnoughSpace")
		return
	}
	// 尾部还剩 7 字节的情况
	q.consumer = 0
	for _, loc := range []uint64{q.Len() - 1, q.Len() - 2, q.Len() - 3, q.Len() - 4, q.Len() - 5, q.Len() - 6, q.Len() - 7} {
		q.producer = loc
		_, _, err = q.Alloc(1)
		if !errors.Is(err, ErrOfNotEnoughSpace) {
			t.Errorf("should be: ErrOfNotEnoughSpace: %+v, loc=%d", err, loc)
			return
		}
	}
	q.producer = 0
	//  尾部还剩 7 字节的情况
	q.consumer = 1
	for _, loc := range []uint64{q.Len() - 1, q.Len() - 2, q.Len() - 3, q.Len() - 4, q.Len() - 5, q.Len() - 6, q.Len() - 7} {
		q.producer = loc
		_, _, err = q.Alloc(1)
		if !errors.Is(err, ErrOfNotEnoughSpace) {
			t.Errorf("should be: ErrOfNotEnoughSpace: %+v, loc=%d", err, loc)
			return
		}
	}
	// 尾部超过 7 字节的情况
	q.consumer = 1
	for _, loc := range []uint64{q.Len() - 8, q.Len() - 9} {
		q.producer = loc
		_, _, err = q.Alloc(10)
		if !errors.Is(err, ErrOfNotEnoughSpace) {
			t.Errorf("should be: ErrOfNotEnoughSpace: %+v, loc=%d", err, loc)
			return
		}
	}
	// head 在最右侧的情况
	q.consumer = q.Len() - 4
	for _, loc := range []uint64{0, 1, 2, 3} {
		q.producer = loc
		_, _, err = q.Alloc(10)
		if err != nil {
			t.Errorf("should be success: %+v, loc=%d", err, loc)
			return
		}
	}
	// 至少要剩余 9 字节才会分配成功
	q.consumer = minLeftLen + 1
	q.producer = 0
	_, _, err = q.Alloc(1)
	if !errors.Is(err, ErrOfNotEnoughSpace) {
		t.Errorf("should be: ErrOfNotEnoughSpace: %+v, loc=%d", err, q.producer)
		return
	}
	// 超过 9 字节会成功
	q.consumer = minLeftLen + 1 + 1
	q.producer = 0
	_, _, err = q.Alloc(1)
	if err != nil {
		t.Errorf("should be success: %+v, loc=%d", err, q.producer)
		return
	}
	// pading = 1
	q.consumer = minLeftLen + 1 + 1 + 1
	q.producer = 1
	_, _, err = q.Alloc(1)
	if err != nil {
		t.Errorf("should be success: %+v, loc=%d", err, q.producer)
		return
	}
	// pading = 2
	q.consumer = minLeftLen + 1 + 1 + 1 + 1
	q.producer = 1 + 1
	_, _, err = q.Alloc(1)
	if err != nil {
		t.Errorf("should be success: %+v, loc=%d", err, q.producer)
		return
	}
	// pading = 3
	q.consumer = minLeftLen + 1 + 1 + 1 + 1 + 1
	q.producer = 1 + 1 + 1
	_, _, err = q.Alloc(1)
	if err != nil {
		t.Errorf("should be success: %+v, loc=%d", err, q.producer)
		return
	}
	// 空间不够，又正好队满
	q.consumer = 0
	q.producer = q.Len() - 8
	_, _, err = q.Alloc(1)
	if !errors.Is(err, ErrOfNotEnoughSpace) {
		t.Errorf("should be: ErrOfNotEnoughSpace: %+v, loc=%d", err, q.producer)
		return
	}
}

func TestRingBufferQueue_GetOne(t *testing.T) {
	posix.ShmUnlink(shmNameOfQueue)
	q, err := NewSpscQueueFromShm(
		shmNameOfQueue,
		queueBytes,
		true,
	)
	if err != nil {
		t.Error(err)
		return
	}
	defer q.Unmap()
	// 队空
	q.consumer = 0
	q.producer = 0
	_, _, err = q.GetOne()
	if !errors.Is(err, ErrOfQueueIsEmpty) {
		t.Errorf("should be: ErrOfQueueIsEmpty: %+v, loc=%d", err, q.producer)
		return
	}
	// head 在最右侧的情况
	q.consumer = 0
	q.producer = 5
	itemLenPtr := (*uint32)(q.GetPointer(0))
	*itemLenPtr = 1
	for _, loc := range []uint64{q.Len() - 1, q.Len() - 2, q.Len() - 3, q.Len() - 4, q.Len() - 5, q.Len() - 7} {
		q.consumer = loc
		_, _, err = q.GetOne()
		if err != nil {
			t.Errorf("should be success: %+v, loc=%d", err, loc)
			return
		}
	}
	// 队满的情况
	q.consumer = 5
	padding := uint64(1)
	q.producer = 0
	itemLenPtr = (*uint32)(q.GetPointer(5 + padding))
	*itemLenPtr = 1
	_, _, err = q.GetOne()
	if err != nil {
		t.Errorf("should be success: %+v, head=%d", err, q.consumer)
		return
	}
	// 尾部空间无效的情况
	q.consumer = 5
	q.producer = 0
	itemLenPtr = (*uint32)(q.GetPointer(5))
	*itemLenPtr = 0
	_, _, err = q.GetOne()
	if err == nil {
		t.Errorf("should be error: %+v, head=%d", err, q.consumer)
		return
	}
	// pading = 0
	q.consumer = 8
	padding = uint64(0)
	q.producer = 0
	itemLenPtr = (*uint32)(q.GetPointer(q.consumer + padding))
	*itemLenPtr = 1
	_, _, err = q.GetOne()
	if err != nil {
		t.Errorf("should be success: %+v, head=%d", err, q.consumer)
		return
	}
	// pading = 1
	q.consumer = 8 - 3
	padding = uint64(1)
	q.producer = 0
	itemLenPtr = (*uint32)(q.GetPointer(q.consumer + padding))
	*itemLenPtr = 1
	_, _, err = q.GetOne()
	if err != nil {
		t.Errorf("should be success: %+v, head=%d", err, q.consumer)
		return
	}
	// pading = 2
	q.consumer = 8 - 2
	padding = uint64(2)
	q.producer = 0
	itemLenPtr = (*uint32)(q.GetPointer(q.consumer + padding))
	*itemLenPtr = 1
	_, _, err = q.GetOne()
	if err != nil {
		t.Errorf("should be success: %+v, head=%d", err, q.consumer)
		return
	}
	// pading = 3
	q.consumer = 8 - 1
	padding = uint64(3)
	q.producer = 0
	itemLenPtr = (*uint32)(q.GetPointer(q.consumer + padding))
	*itemLenPtr = 1
	_, _, err = q.GetOne()
	if err != nil {
		t.Errorf("should be success: %+v, head=%d", err, q.consumer)
		return
	}
}

func runProducerAndConsumer(q *SpscQueue) {
	const end = 10000000
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		// producer
		start := uint64(0)
		for start < end {
			buf, newTail, err := q.Alloc(8)
			if err != nil {
				if errors.Is(err, ErrOfNotEnoughSpace) {
					//SchedYield()
					runtime.Gosched()
					continue
				}
				log.Println("alloc err:", err)
				return
			}
			binary.LittleEndian.PutUint64(buf, start)
			start++
			//log.Println(start)
			if !q.CommitProduce(newTail) {
				log.Println("CommitProduce fail")
				return
			}
		}
	}()
	go func() {
		defer wg.Done()
		// comsumer
		want := uint64(0)
		for want != end {
			buf, newHead, err := q.GetOne()
			if err != nil {
				if errors.Is(err, ErrOfQueueIsEmpty) {
					//SchedYield()
					runtime.Gosched()
					continue
				}
				log.Println("get one err:", err)
				return
			}
			if binary.LittleEndian.Uint64(buf) != want {
				log.Println("num not match:", want)
				return
			}
			want++
			if !q.CommitConsume(newHead) {
				log.Println("CommitConsume fail")
				return
			}
		}
	}()
	wg.Wait()
}

func TestShmQueueForNumber(t *testing.T) {
	posix.ShmUnlink(shmNameOfQueue)
	q, err := NewSpscQueueFromShm(
		shmNameOfQueue,
		queueBytes,
		true,
	)
	if err != nil {
		t.Error(err)
		return
	}
	defer q.Unmap()
	//
	runProducerAndConsumer(q)
}

func Test_NewSpscQueue_from_heap(t *testing.T) {
	q, err := NewSpscQueue(4095)
	if !errors.Is(err, ErrOfBadQueueSize) {
		t.Error("should be ErrOfBadQueueSize")
		return
	}
	q, err = NewSpscQueue(4096)
	if err != nil {
		t.Error("err=", err)
		return
	}
	ptr := uint64(uintptr(unsafe.Pointer(q)))
	if ptr%8 != 0 {
		t.Errorf("should be align to 8")
		return
	}
	runProducerAndConsumer(q)
}
