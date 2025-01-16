package spscqueue

import "errors"

// errors for NewSpscQueueFromShm
var (
	ErrOfBadQueueSize       = errors.New("bad queue size")
	ErrOfOpenShmFail        = errors.New("open shm fail")
	ErrOfShmNotExist        = errors.New("shm not exist")
	ErrOfCreateShmFail      = errors.New("create shm fail")
	ErrOfTruncateShm        = errors.New("truncate error")
	ErrOfMMapError          = errors.New("mmap error")
	ErrOfBadMaskOfOpenedShm = errors.New("bad mask of opened shm")
)

// errors for Alloc/GetOne
var (
	ErrOfBadParam          = errors.New("bad param")
	ErrOfBytesTooLarge     = errors.New("bytes too large")
	ErrOfNotEnoughSpace    = errors.New("not enough space")
	ErrOfCommitProduceFail = errors.New("commit produce fail")
	ErrOfQueueIsEmpty      = errors.New("queue is empty")
	ErrOfCommitConsumeFail = errors.New("commit consume fail")
)
