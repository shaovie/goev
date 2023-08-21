package goev

import (
	"sync"
	"sync/atomic"
	"time"
)

// Use sync.Pool for underlying memory reuse and employs finer-grained granularity management.

const (
	bufPoolMaxMBytes   int   = 16
	bufPoolActiveTimes int32 = 5 // Active is enable, inactive is disable, periodic dynamic adjustment
)

var (
	// 0 [1,       128*1] 128*1
	// 1 [128*1+1, 128*2] 128*2
	// 2 [128*2+1, 128*3] 128*3
	// ...
	// 6 [128*6+1, 128*7] 128*7
	btPool [7]buffPool

	// 0 [128*7+1,   1024*1] 1K
	// 1 [1024*1+1,  1024*2] 2K
	// 2 [1024*2+1), 1024*3] 3K
	// ...
	// 1022 [1024*1022+1), 1024*1023] 1023K
	kbPool [1023]buffPool

	// 0  [1024*1023+1,    1024*1024*1]  1M
	// 1  [1024*1024*1+1,  1024*1024*2]  2M
	// 2  [1024*1024*2+1,  1024*1024*3]  3M
	// ...
	// 15 (1024*1024*15+1, 1024*1024*16] 16M
	mbPool [bufPoolMaxMBytes]buffPool // 1024K x n  <= 16M
)

func init() {
	for i := 0; i < len(btPool); i++ {
		btPool[i].init((i + 1) * 128)
	}
	for i := 0; i < len(kbPool); i++ {
		kbPool[i].init((i + 1) * 1024)
	}
	for i := 0; i < len(mbPool); i++ {
		mbPool[i].init((i + 1) * 1024 * 1024)
	}
	go buffPoolAdjustTiming(time.Minute * 2)
}

// BMalloc allocates memory of the specified size and maximizes the utilization of memory reuse
//
// Returned []byte, with len representing the actual requested size,
// and cap being greater than or equal to the size.
// NOTE the []byte buffer cannot use 'append'
func BMalloc(s int) []byte {
	if s < 1 {
		return make([]byte, 0)
	}

	if s < (128*7 + 1) {
		i := (s+127)/128 - 1
		return btPool[i].alloc(s)
	} else if s < (1024*1024 + 1) {
		i := (s+1023)/1024 - 1
		return kbPool[i].alloc(s)
	} else if s < (1024*1024*bufPoolMaxMBytes + 1) {
		i := (s+(1024*1024-1))/(1024*1024) - 1
		return mbPool[i].alloc(s)
	}
	return make([]byte, s)
}

// BFree free memory
func BFree(bf []byte) {
	s := cap(bf)
	if s < (128*7 + 1) {
		i := (s+127)/128 - 1
		btPool[i].free(s, bf)
	} else if s < (1024*1024 + 1) {
		i := (s+1023)/1024 - 1
		kbPool[i].free(s, bf)
	} else if s < (1024*1024*bufPoolMaxMBytes + 1) {
		i := (s+(1024*1024-1))/(1024*1024) - 1
		mbPool[i].free(s, bf)
	}
}
func buffPoolAdjustTiming(period time.Duration) {
	ticker := time.NewTicker(period)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			buffPoolAdjust()
		}
	}
}
func buffPoolAdjust() {
	for i := 0; i < len(btPool); i++ {
		btPool[i].adjust()
	}
	for i := 0; i < len(kbPool); i++ {
		kbPool[i].adjust()
	}
	for i := 0; i < len(mbPool); i++ {
		mbPool[i].adjust()
	}
}

type buffPool struct {
	disabled   int32
	allocTimes int32
	size       int
	pool       sync.Pool
}

func (bp *buffPool) init(size int) {
	bp.size = size
	bp.pool.New = func() any { return make([]byte, size) }
}
func (bp *buffPool) alloc(s int) []byte {
	atomic.AddInt32(&(bp.allocTimes), 1)
	if atomic.LoadInt32(&(bp.disabled)) == 1 {
		return make([]byte, s)
	}
	bf := bp.pool.Get().([]byte)
	if s == bp.size {
		return bf
	}
	return bf[:s]
}
func (bp *buffPool) free(c int, bf []byte) {
	if c == bp.size {
		if len(bf) < c {
			bp.pool.Put(bf[:c])
			return
		}
		bp.pool.Put(bf)
	}
}
func (bp *buffPool) adjust() {
	if atomic.SwapInt32(&(bp.allocTimes), 0) < bufPoolActiveTimes {
		atomic.CompareAndSwapInt32(&(bp.disabled), 0, 1)
	} else {
		atomic.CompareAndSwapInt32(&(bp.disabled), 1, 0)
	}
}
