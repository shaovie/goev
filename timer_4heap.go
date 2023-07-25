package goev

import (
	"errors"
	"sync"
	"time"
)

type timer4Heap struct {
	noCopy

	fheap    []*timerItem
	fheapMtx sync.Mutex
}

func newTimer4Heap(initCap int) *timer4Heap {
	if initCap < 1 {
		panic("timer4Heap initCap invalid!")
	}

	th := &timer4Heap{
		fheap: make([]*timerItem, 0, initCap),
	}
	return th
}

func (th *timer4Heap) schedule(eh EvHandler, delay, interval int64) error {
	if delay < 0 || interval < 0 {
		return errors.New("params are invalid")
	}
	if eh.getTimerItem() != nil {
		return errors.New("eh had scheduled")
	}

	now := time.Now().UnixMilli()
	ti := &timerItem{
		expiredAt: now + delay,
		interval:  interval,
		eh:        eh,
	}
	th.fheapMtx.Lock()
	th.fheap = append(th.fheap, ti)
	th.shiftUp(len(th.fheap) - 1)
	eh.setTimerItem(ti)
	th.fheapMtx.Unlock()
	return nil
}
func (th *timer4Heap) scheduleTest(eh EvHandler, delay, interval int64) error {
	ti := &timerItem{
		expiredAt: delay,
		interval:  interval,
		eh:        eh,
	}
	eh.setTimerItem(ti)
	th.fheapMtx.Lock()
	th.fheap = append(th.fheap, ti)
	th.shiftUp(len(th.fheap) - 1)
	th.fheapMtx.Unlock()
	return nil
}
func (th *timer4Heap) cancel(eh EvHandler) {
	ti := eh.getTimerItem()
	if ti == nil {
		return
	}
	th.fheapMtx.Lock()
	ti.eh = nil // TODO eh atomic.Value ?
	th.fheapMtx.Unlock()
}
func (th *timer4Heap) handleExpired(now int64) int64 {
	th.fheapMtx.Lock()
	defer th.fheapMtx.Unlock()

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
		if item.eh == nil { // canceled
			continue
		}
		if item.eh.OnTimeout(now) == true && item.interval > 0 {
			item.expiredAt = now + item.interval
			th.fheap = append(th.fheap, item)
			th.shiftUp(len(th.fheap) - 1)
		}
	}
	return delta
}

func (th *timer4Heap) size() int {
	th.fheapMtx.Lock()
	defer th.fheapMtx.Unlock()
	return len(th.fheap)
}

func (th *timer4Heap) popOne(now, errorVal int64) (*timerItem, int64) {
	if len(th.fheap) == 0 {
		return nil, 0
	}

	min := th.fheap[0]
	delta := min.expiredAt - now
	if delta > errorVal {
		return nil, delta
	}
	last := len(th.fheap) - 1
	th.fheap[0] = th.fheap[last]
	th.fheap = th.fheap[:last]

	th.shiftDown(0)

	return min, 0
}

func (th *timer4Heap) shiftUp(index int) {
	parent := (index - 1) / 4

	for index > 0 && th.fheap[index].expiredAt < th.fheap[parent].expiredAt {
		th.fheap[index], th.fheap[parent] = th.fheap[parent], th.fheap[index]
		index = parent
		parent = (index - 1) / 4
	}
}

func (th *timer4Heap) shiftDown(index int) {
	size := len(th.fheap)

	for {
		smallest := index
		childStart := 4*index + 1
		childEnd := childStart + 4

		if childStart < size {
			for i := childStart; i < childEnd && i < size; i++ {
				if th.fheap[i].expiredAt < th.fheap[smallest].expiredAt {
					smallest = i
				}
			}

			if smallest != index {
				th.fheap[index], th.fheap[smallest] = th.fheap[smallest], th.fheap[index]
				index = smallest
			} else {
				break
			}
		} else {
			break
		}
	}
}
