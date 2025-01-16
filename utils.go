package spscqueue

import "math/bits"

// IsPowerOfTwo checks if a uint64 number is a power of two
func IsPowerOfTwo(n uint64) bool {
	// A number is a power of two if it is greater than 0
	// and has only one bit set in its binary representation
	return n > 0 && (n&(n-1)) == 0
}

// RoundPowerOfTwo 找到与 n 最邻近且大于等于 n 的 2 的幂 的数
func RoundPowerOfTwo(n uint64) uint64 {
	if n < minQueueBytes {
		return minQueueBytes
	}
	if IsPowerOfTwo(n) {
		return n
	}
	cnt := bits.LeadingZeros64(n)
	return 1 << (64 - cnt)
}
