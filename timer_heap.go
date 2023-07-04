package goev

import (
	"time"
    "sync"
	"errors"
	"container/heap"
)

type minHeap []*timerItem

// heap Interface
func (mh minHeap) Len() int { return len(mh) }

// heap Interface
func (mh minHeap) Less(i, j int) bool {
	return mh[i].expiredAt < mh[j].expiredAt
}

// heap Interface
func (mh minHeap) Swap(i, j int) {
	mh[i], mh[j] = mh[j], mh[i]
}

// heap Interface
func (mh *minHeap) Push(x any) {
	item := x.(*timerItem)
	*mh = append(*mh, item)
}

// heap Interface
func (mh *minHeap) Pop() any {
	old := *mh
	n := len(old)
	item := old[n-1]
	old[n-1] = nil  // avoid memory leak
	*mh = old[0 : n-1]
	return item
}

//= timer heap
type timerHeap struct {
    noCopy

    heap minHeap
    heapMtx sync.Mutex
}
func newTimerHeap(initCap int) *timerHeap {
    if initCap < 1 {
        panic("timerHeap initCap invalid!")
    }

    th := &timerHeap{
        heap: make(minHeap, 0, initCap),
    }
    return th
}

// in msec
func (th *timerHeap) schedule(eh EvHandler, delay, interval int64) error {
    if delay < 0 || interval < 0 {
        return errors.New("params are invalid!")
    }

    now := time.Now().UnixMilli()
    ti := &timerItem{
        expiredAt: now + delay,
        interval: interval,
        eh: eh,
    }
    th.heapMtx.Lock()
    heap.Push(&th.heap, ti)
    th.heapMtx.Unlock()
    return nil
}
func (th *timerHeap) scheduleTest(eh EvHandler, delay, interval int64) error {
    ti := &timerItem{
        expiredAt: delay,
        interval: interval,
        eh: eh,
    }
    th.heapMtx.Lock()
    heap.Push(&th.heap, ti)
    th.heapMtx.Unlock()
    return nil
}
func (th *timerHeap) handleExpired(now int64) int64 {
    th.heapMtx.Lock()
    defer th.heapMtx.Unlock()
     
    delta := int64(-1)
    var item *timerItem
    for {
        item, delta = th.popOne(now, 2)
        if item == nil {
            if delta == 0 { // empty
                delta = -1
            }
            break
        }
        if item.eh.OnTimeout(now) == true && item.interval > 0 {
            item.expiredAt = now + item.interval
            heap.Push(&th.heap, item)
        }
    }
    return delta
}
func (th *timerHeap) size() int {
    th.heapMtx.Lock()
    defer th.heapMtx.Unlock()
    return len(th.heap)
}
func (th *timerHeap) popOne(now, errorVal int64) (*timerItem, int64) {
    if len(th.heap) == 0 {
		return nil, 0
	}

	min := th.heap[0]
    delta := min.expiredAt - now
    if delta > errorVal { // The error is errorVal
		return nil, delta
	}
	heap.Remove(&th.heap, 0)
	return min, 0
}
