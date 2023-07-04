package goev

import (
	"time"
    "sync"
	"errors"
)

type timerItem struct {
    noCopy

    expiredAt int64
    interval  int64

    eh EvHandler
}

//= timer item
// opt: sync.Pool
func newTimerItem(expiredAt, interval int64, eh EvHandler) *timerItem {
    return &timerItem{
        expiredAt: expiredAt,
        interval: interval,
        eh: eh,
    }
}

//= timer heap
type timerHeap struct {
    noCopy

    pq PriorityQueue
    pqMtx sync.Mutex
}
func newTimerHeap(initCap int) *timerHeap {
    if initCap < 1 {
        panic("timerHeap initCap invalid!")
    }

    th := &timerHeap{
        pq: NewPriorityQueue(initCap),
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
    th.pqMtx.Lock()
    th.pq.PushOne(NewPriorityQueueItem(ti, ti.expiredAt))
    th.pqMtx.Unlock()
    return nil
}
func (th *timerHeap) cancel(eh EvHandler) {
    //timerId := eh.GetTimerId()
}
func (th *timerHeap) handleExpired(now int64) int64 {
    th.pqMtx.Lock()
    defer th.pqMtx.Unlock()
     
    delta := int64(-1)
    var item *PriorityQueueItem
    for {
        item, delta = th.pq.PopOne(now, 2)
        if item == nil {
            if delta == 0 { // empty
                delta = -1
            }
            break
        }
        ti := item.Value.(*timerItem)
        if ti.eh.OnTimeout(now) == true && ti.interval > 0 {
            ti.expiredAt = now + ti.interval
            th.pq.PushOne(NewPriorityQueueItem(ti, ti.expiredAt))
        }
    }
    return delta
}
