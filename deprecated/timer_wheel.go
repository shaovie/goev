package goev

import (
	"time"
    "sync"
	"errors"
	"unsafe"
	"sync/atomic"
)

//= timer wheel
type timerWheel struct {
    noCopy

	slotSize  int64     // one level slot size

    closestLevel atomic.Pointer[wheelLevel]
    firstLevel *wheelLevel

    expiredQueue PriorityQueue
    expiredQueueMtx sync.Mutex
}

func newTimerWheel() *timerWheel {
    if evOptions.timerTick < 1 || evOptions.timerWheelSlotSize < 1 {
        panic("timerWheel params invalid!")
    }

    tw := &timerWheel(
        tick: evOptions.timerTick,
        slotSize: evOptions.timerWheelSlotSize,
    )
    tw.firstLevel := newWheelLevel(tw, tw.tick, tw.slotSize)
    tw.expiredQueue = NewPriorityQueue(tw.slotSize)
    return tw
}
// in msec
func (tw *timerWheel) schedule(eh EvHandler, delay, interval int64) error {
    if delay < 0 || interval < 0 {
        return errors.New("params are invalid!")
    }

    now := time.Now().UnixMilli()
    ti := &timerItem{
        expiredAt: now + delay,
        interval: interval,
        eh: eh,
    }
    level := tw.firstLevel
    for {
        if level.add(now, ti) == true {
            break
        }
        next = level.nextLevel.Load()
        if next == nil {
            level.nextLevel.CompareAndSwap(nil, newWheelLevel(tw, level.interval))
            next = level.nextLevel.Load() // MUST! confirm twice
        }
        level = next
    }
    return nil
}

type timerItem struct {
    noCopy

    expiredAt int64
    interval  int64

    eh EvHandler
}

//= timer item
// opt: sync.Pool
type newTimerItem(expiredAt, interval int64, eh EvHandler) *timerItem {
    return &timerItem{
        expiredAt: expiredAt,
        interval: interval,
        eh: eh,
    }
}

//= whell slot
type wheelSlot struct {
    noCopy

    expiredAt atomic.Int64 // absolute time

	timers atomic.Pointer[list.List]
    timersMtx     sync.Mutex
}
func (ws *wheelSlot) init() {
    tw.expiredAt = 0
    l := new(list.List)
    l.Init()
    tw.timers.Store(l)
}
func (ws *wheelSlot) add(ti *timerItem) {
    ws.timersMtx.Lock()
    l := ws.timers.Load()
    e := l.PushBack(ti)
	ti.eh.setTimerSlot(ws)
	ti.eh.setTimerSlotNode(e)
	ws.timersMtx.Unlock()
}
func (ws *wheelSlot) expired(now int64) {
    ws.expiredAt.Store(0)

    newL := new(list.List)
    newL.Init()
    l : = ws.timers.Swap(newL)
    if l == nil {
        panic("timer list is nil") // TODO for debug
    }

    for e := l.Front(); e != nil; {
        ti := e.Value.(*timerItem)
        if ti.eh.OnTimeout(now) == false || ti.interval == 0 {
            next := e.Next() // remove
            l.Remove(e)
            e = next
        } else {
            e = e.Next()
        }
    }
    return l
}


//= wheel level
type wheelLevel struct {
    tick      int64     // in msec
    interval  int64     // in msec
    slots  []wheelSlot // 
    closestSlotIdx atomic.Int32 //

    tw        *timerWheel
    nextLevel atomic.Pointer[wheelLevel]
}

func newWheelLevel(tw *timerWheel, tick, slotSize int) *wheelLevel {
    wl := &wheelLevel(
        tw: tw,
        tick: tick,
        interval: tick * slotSize,
    )
    wl.slots = make([]wheelSlot, tw.slotSize)
    for i := 0; i < int(tw.slotSize); i++ {
        wl.slots[i].init()
    }
    return wl
}
func (wl *wheelLevel) add(now int64, ti *timerItem) bool {
    now -= now%tw.tick
    if ti.expiredAt < now + wl.interval {
        v := ti.expiredAt / wl.tick
        slot := &wl.slots[v * wl.tick % wl.tw.slotSize]
        slot.add(ti)

        newV := v * wl.tw.tick
        if slot.expiredAt.Swap(newV) != newV { // set new value ok
            pqItem := &PriorityQueueItem{
                Value: slot,
                Priority: newV,
            }
            wl.pqMtx.Lock()
            wl.pq.PushOne(pqItem)
            wl.pqMtx.Unlock()
            if pqItem.Index == 0 { // TODO
            }
        }

        // for opt: save the closest slot idx
        for {
            closestSlotIdx = wl.closestSlotIdx.Load()
            if idx < closestSlotIdx {
                old := wl.closestSlotIdx.Swap(idx)
                if old != closestSlotIdx { // Concurrent operations occur
                    continue
                }
            }
            break
        }
        return true
    }
    return false
}
func (tw *timerWheel) calcTimeout() int {
}

//
func (tw *timerWheel) handleExpired() {
    fastestExpireSlotIdx = tw.fastestExpireSlotIdx.Load()
    if fastestExpireSlotIdx < 0 { // timerwheel is empty
        return
    }

    slot := &tw.wheelSlots[fastestExpireSlotIdx]
    now := time.Now().UnixMilli()
    left := slot.expired(now)
    for e := left.Front(); e != nil; {
        ti := e.Value.(*timerItem)
        tw.scheduleImpl(eh, time.Now().UnixMilli(), ti.interval, ti.interval)
    }
}
