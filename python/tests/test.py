import sys
import os
import ctypes

# 获取当前脚本所在目录的路径
current_dir = os.path.dirname(os.path.abspath(__file__))

# 获取上级目录的路径
parent_dir = os.path.dirname(current_dir)

# 获取上级目录中的目标目录路径
target_dir = os.path.join(parent_dir, './spscqueue/')

# 将目标目录添加到 sys.path
sys.path.append(target_dir)

from spscqueue import *

def main():
    q = SpscQueue()
    q.NewSpscQueueFromShm("test_1", 1024*4, True)
    print(dir(q))
    print("len:", q.Len())
    print("is full:", q.IsFull())
    print("is empty:", q.IsEmpty())
    # 测试分配
    testStr = "it's a test".encode("utf-8")
    length = len(testStr)
    ret, buf, newTail = q.Alloc(length)
    if ret!=0:
        print("alloc failed, ret=", ret)
        return
    print(type(newTail), newTail, repr(newTail))
    ctypes.memmove(buf, testStr, length)
    # 提交生产
    if not q.CommitProduce(newTail):
        print("commit produce failed")
        return
    # 消费
    ret, outBuf, outLength, newHead = q.GetOne()
    if ret!=0:
        print("get one failed, ret=", ret)
        return
    if length!=outLength:
        print("length not match, length=", length, "outLength=", outLength)
        return
    buffer = ctypes.create_string_buffer(outLength+1)
    ctypes.memmove(buffer, outBuf, outLength)
    buffer[outLength] = 0
    pointer = ctypes.c_void_p(ctypes.addressof(buffer))
    outStr = ctypes.string_at(pointer)
    print(outStr)
    if outStr!=testStr:
        print("outStr not match, outStr=", outStr, "testStr=", testStr)
        return
    # 提交消费
    if not q.CommitConsume(newHead):
        print("commit consume failed")
        return

if __name__ == "__main__":
    main()
