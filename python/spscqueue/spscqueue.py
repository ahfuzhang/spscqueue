import sys
import os
import ctypes

class SpscQueue:
    def __init__(self):
        uname = os.uname()
        current_dir = os.path.dirname(os.path.abspath(__file__))
        soName = f"./spsc_queue_{uname.sysname}_{uname.machine}.so"
        parent_dir = os.path.dirname(current_dir)
        target_dir = os.path.join(parent_dir, './spscqueue/', soName)
        self.lib = ctypes.CDLL(target_dir)  # 如果 so 不存在，抛出异常 OSError
        # 声明函数类型
        self.lib.NewSpscQueueFromShm.argtypes = [
            ctypes.c_char_p,  # shmName
            ctypes.c_uint64,  # queueSize
            ctypes.c_bool,  # isCreate
            ctypes.c_void_p, # int *outErrorNumber
            ctypes.c_void_p  # SpscQueue **outQueue
        ]
        self.lib.NewSpscQueueFromShm.restype = ctypes.c_int
        #
        self.lib.Alloc.argtypes = [
            ctypes.c_void_p, # SpscQueue *q
            ctypes.c_uint32, # uint32_t bytes
            ctypes.c_void_p, # void **outPtr
            ctypes.c_void_p  # uint64_t *outNewTail
        ]
        self.lib.Alloc.restype = ctypes.c_int
        #
        self.lib.CommitProduce.argtypes = [ctypes.c_void_p, ctypes.c_uint64]
        self.lib.CommitProduce.restype = ctypes.c_bool
        #
        self.lib.Produce.argtypes = [ctypes.c_void_p, ctypes.c_void_p, ctypes.c_uint32]
        self.lib.Produce.restype = ctypes.c_bool
        #
        self.lib.GetOne.argtypes = [ctypes.c_void_p, ctypes.c_void_p, ctypes.c_void_p, ctypes.c_void_p]
        self.lib.GetOne.restype = ctypes.c_int
        #
        self.lib.IsPowerOfTwo.argtypes = [ctypes.c_uint64]
        self.lib.IsPowerOfTwo.restype = ctypes.c_bool
        #
        self.lib.CommitConsume.argtypes = [ctypes.c_void_p, ctypes.c_uint64]
        self.lib.CommitConsume.restype = ctypes.c_bool
        #
        self.lib.Consume.argtypes = [ctypes.c_void_p, ctypes.c_void_p, ctypes.c_void_p]
        self.lib.Consume.restype = ctypes.c_int
        #
        self.lib.RoundPowerOfTwo.argtypes = [ctypes.c_uint64]
        self.lib.RoundPowerOfTwo.restype = ctypes.c_uint64
        #
        self.lib.Len.argtypes = [ctypes.c_void_p]
        self.lib.Len.restype = ctypes.c_uint64
        #
        self.lib.IsFull.argtypes = [ctypes.c_void_p]
        self.lib.IsFull.restype = ctypes.c_bool
        #
        self.lib.IsEmpty.argtypes = [ctypes.c_void_p]
        self.lib.IsEmpty.restype = ctypes.c_bool
        #
        self.lib.Unmap.argtypes = [ctypes.c_void_p]
        self.q = ctypes.c_void_p(0)

    def NewSpscQueueFromShm(self, shmName, queueBytes, isCreate):
        errno = ctypes.c_int(0)
        ret = self.lib.NewSpscQueueFromShm(
            shmName.encode("utf-8"),
            queueBytes,
            isCreate,
            ctypes.byref(errno),
            ctypes.byref(self.q)
        )
        if ret!= 0:
            raise Exception(f"NewSpscQueueFromShm failed, errno: {errno.value}, ret={ret}")
    def IsPowerOfTwo(self, n):
        return self.lib.IsPowerOfTwo(n)
    def RoundPowerOfTwo(self, n):
        return self.lib.RoundPowerOfTwo(n)
    def Len(self):
        return self.lib.Len(self.q)
    def IsFull(self):
        return self.lib.IsFull(self.q)
    def IsEmpty(self):
        return self.lib.IsEmpty(self.q)
    def Unmap(self):
        self.lib.Unmap(self.q)
    def Alloc(self, needBytes):
        buf = ctypes.c_void_p(0)
        newTail = ctypes.c_uint64(0)
        ret = self.lib.Alloc(self.q, needBytes, ctypes.byref(buf), ctypes.byref(newTail))
        return int(ret), buf, newTail.value
    def CommitProduce(self, newTail):
        return self.lib.CommitProduce(self.q, ctypes.c_uint64(newTail))
    def Produce(self, buf, length):
        if type(buf) != type(ctypes.c_void_p):
            raise Exception("Produce: buf must be ctypes.c_void_p")
        ret = self.lib.Produce(self.q, buf, ctypes.c_uint32(length))
        return int(ret)
    def GetOne(self):
        buf = ctypes.c_void_p(0)
        length = ctypes.c_uint32(0)
        newHead = ctypes.c_uint64(0)
        ret = self.lib.GetOne(self.q, ctypes.byref(buf), ctypes.byref(length), ctypes.byref(newHead))
        return int(ret), buf, length.value, newHead.value
    def CommitConsume(self, newHead):
        return self.lib.CommitConsume(self.q, ctypes.c_uint64(newHead))
    def Consume(self, buf, length):
        if type(buf)!= type(ctypes.c_void_p):
            raise Exception("Comsume: buf must be ctypes.c_void_p")
        outLen = ctypes.c_uint32(length)
        ret = self.lib.Comsume(self.q, buf, ctypes.byref(outLen))
        return int(ret), int(outLen)
