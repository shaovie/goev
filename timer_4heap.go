package goev

import (
	"errors"
	"syscall"
	"time"
	"unsafe"

	"golang.org/x/sys/unix"
)

type timerItem struct {
	noCopy
	expiredAt int64
	interval  int64
	eh        EvHandler
}

type timer4Heap struct {
	IOHandle

	tfd            int
	timerfdSettime int64
	fheap          []*timerItem
}

func newTimer4Heap(initCap int) *timer4Heap {
	if initCap < 1 {
		panic("timer4Heap initCap invalid!")
	}

	tfd, err := unix.TimerfdCreate(unix.CLOCK_BOOTTIME, unix.TFD_NONBLOCK|unix.TFD_CLOEXEC)
	if err != nil {
		if err == unix.ENOSYS {
			panic("timerfd_create system call not implemented")
		}
		panic("TimerfdCreate: " + err.Error())
	}
	th := &timer4Heap{
		tfd:   tfd,
		fheap: make([]*timerItem, 0, initCap),
	}
	return th
}

func (th *timer4Heap) timerfd() int {
	return th.tfd
}
func (th *timer4Heap) adjustTimerfd(delay /*millisecond*/ int64) {
	timeSpec := unix.ItimerSpec{
		Value: unix.NsecToTimespec(delay * 1000 * 1000),
	}
	unix.TimerfdSettime(th.tfd, 0 /*Relative time*/, &timeSpec, nil)
}
func (th *timer4Heap) OnRead(fd int) bool {
	var readTimerfdV int64 = 0 // Compared to var bf [8] byte, the performance is the same
	var readTimerfdBuf = (*(*[8]byte)(unsafe.Pointer(&readTimerfdV)))[:]
	syscall.Read(fd, readTimerfdBuf)
	delay := th.handleExpired(time.Now().UnixMilli())
	if delay > 0 {
		th.adjustTimerfd(delay)
	}
	return true
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
	th.fheap = append(th.fheap, ti)
	th.shiftUp(len(th.fheap) - 1)
	eh.setTimerItem(ti)

	min := th.fheap[0]
	if min.expiredAt != th.timerfdSettime {
		th.adjustTimerfd(min.expiredAt - now)
		th.timerfdSettime = min.expiredAt
	}

	return nil
}
func (th *timer4Heap) scheduleTest(eh EvHandler, delay, interval int64) error {
	ti := &timerItem{
		expiredAt: delay,
		interval:  interval,
		eh:        eh,
	}
	th.fheap = append(th.fheap, ti)
	th.shiftUp(len(th.fheap) - 1)
	eh.setTimerItem(ti)
	return nil
}
func (th *timer4Heap) cancel(eh EvHandler) {
	ti := eh.getTimerItem()
	if ti == nil {
		return
	}
	ti.eh = nil
	ti.expiredAt = 1 // 防止定时器时间太久导致ti回收被延迟太久(这是不确定的, 因为没有改变ti 在heap的位置)
	// No need to adjust timerfd
	eh.setTimerItem(nil)
}
func (th *timer4Heap) handleExpired(now int64) int64 {
	if len(th.fheap) == 0 {
		return 0
	}

	delta := int64(-1)
	var item *timerItem
	for {
		item, delta = th.popOne(now, 2) // 2 是误差范围 表示在0~2之间到期的都会马上执行
		if item == nil {
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
