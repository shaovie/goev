package goev

import (
	"errors"
	"sync/atomic"
	"syscall"

	"github.com/shaovie/goev/netfd"
)

// IOHandle is the base class of io event handling objects
type IOHandle struct {
	noCopy

	asyncWriteWaiting bool
	fd                int32
	asyncWriteBufSize int // total size of wait to write

	r              *Reactor
	ep             *evPoll
	ti             *timerItem
	asyncWriteBufQ *RingBuffer[asyncWriteBuf] // 保存未直接发送完成的
}

// Init IOHandle must be called when reusing it.
func (h *IOHandle) Init() {
	h.r, h.ep, h.ti = nil, nil, nil
	h.setFd(-1)
}

func (h *IOHandle) setParams(fd int, ep *evPoll) {
	h.setFd(fd)
	h.ep = ep
}

func (h *IOHandle) getEvPoll() *evPoll {
	return h.ep
}

func (h *IOHandle) setReactor(r *Reactor) {
	h.r = r
}

// GetReactor can retrieve the current event object bound to which Reactor
func (h *IOHandle) GetReactor() *Reactor {
	return h.r
}

func (h *IOHandle) setTimerItem(ti *timerItem) {
	h.ti = ti
}

func (h *IOHandle) getTimerItem() *timerItem {
	return h.ti
}

// Fd return fd
func (h *IOHandle) Fd() int {
	return int(atomic.LoadInt32(&(h.fd)))
}
func (h *IOHandle) setFd(fd int) {
	atomic.StoreInt32(&(h.fd), int32(fd))
}

// ScheduleTimer Add a timer event to an IOHandle that is already registered with the reactor
// to ensure that all event handling occurs within the same evpoll
//
// Only supports binding timers to I/O objects within evpoll internally.
func (h *IOHandle) ScheduleTimer(eh EvHandler, delay, interval int64) error {
	if h.ep != nil {
		return h.ep.scheduleTimer(eh, delay, interval)
	}
	return errors.New("ev handler has not been added to the reactor yet")
}

// CancelTimer cancels a timer that has been successfully scheduled
func (h *IOHandle) CancelTimer(eh EvHandler) {
	if h.ep != nil {
		h.ep.cancelTimer(eh)
	}
}

// Read use evPollReadBuff, buf size can set by options.EvPollReadBuffSize
//
// Can only be used within the poller goroutine
func (h *IOHandle) Read() (bf []byte, n int, err error) {
	fd := h.Fd()
	if fd < 1 {
		return nil, 0, syscall.EBADF
	}
	if h.ep != nil {
		return h.ep.read(fd)
	}
	panic("goev: IOHandle.Read fd not register to evpoll")
}

// WriteBuff must be registered with evpoll in order to be used
//
// Can only be used within the poller goroutine
func (h *IOHandle) WriteBuff() []byte {
	if h.ep != nil {
		return h.ep.writeBuff()
	}
	panic("goev: IOHandle.WriteBuff fd not register to evpoll")
}

// PCachedGet returns cached data store in evPoll, it's lock free
func (h *IOHandle) PCachedGet(id int) (any, bool) {
	return h.ep.pCacheGet(id)
}

// Write synchronous write.
// n = [0, len(bf)]
func (h *IOHandle) Write(bf []byte) (n int, err error) {
	fd := h.Fd()
	if fd < 1 { // NOTE fd must > 0
		return 0, syscall.EBADF
	}
	if h.asyncWriteBufQ != nil && !h.asyncWriteBufQ.IsEmpty() {
		abf := ioAllocBuff(len(bf))
		n = copy(abf, bf)
		h.asyncWriteBufQ.PushBack(asyncWriteBuf{
			len: n,
			buf: abf,
		})
		h.asyncWriteBufSize += n
		return
	}
	for {
		n, err = syscall.Write(fd, bf)
		if n < 0 {
			if err == syscall.EINTR {
				continue
			}
			n = 0
		}
		break
	}
	if n < len(bf) {
		abf := ioAllocBuff(len(bf) - n)
		n = copy(abf, bf[n:])
		if h.asyncWriteBufQ == nil {
			h.asyncWriteBufQ = NewRingBuffer[asyncWriteBuf](2)
		}
		h.asyncWriteBufQ.PushBack(asyncWriteBuf{
			len: n,
			buf: abf,
		})
		h.asyncWriteBufSize += n
		if h.asyncWriteWaiting == false {
			h.asyncWriteWaiting = true
			h.ep.append(fd, EvOut) // No need to use ET mode
			// eh needs to implement the OnWrite method, and the OnWrite method
			// needs to call AsyncOrderedFlush.
		}
		n = len(bf)
	}
	return
}

// Destroy If you are using the Async write mechanism, it is essential to call the Destroy method
// in OnClose to clean up any unsent bf data.
func (h *IOHandle) Destroy(eh EvHandler) {
	fd := h.Fd()
	if fd > 0 {
		netfd.Close(fd)
		h.setFd(-1)
	}

	if h.asyncWriteBufQ != nil && !h.asyncWriteBufQ.IsEmpty() {
		for {
			abf, ok := h.asyncWriteBufQ.PopFront()
			if !ok {
				break
			}
			ioFreeBuff(abf.buf)
		}
	}
}

//
//= EvHandler interface

// OnOpen please make sure you want to reimplement it.
func (*IOHandle) OnOpen() bool {
	panic("goev: IOHandle OnOpen")
}

// OnRead please make sure you want to reimplement it.
func (*IOHandle) OnRead() bool {
	panic("goev: IOHandle OnRead")
}

// OnWrite please make sure you want to reimplement it.
func (*IOHandle) OnWrite() bool {
	panic("goev: IOHandle OnWrite")
}

// OnConnectFail please make sure you want to reimplement it.
func (*IOHandle) OnConnectFail(err error) {
	panic("goev: IOHandle OnConnectFail")
}

// OnTimeout please make sure you want to reimplement it.
func (*IOHandle) OnTimeout(millisecond int64) bool {
	panic("goev: IOHandle OnTimeout")
}

// OnClose please make sure you want to reimplement it.
func (*IOHandle) OnClose() {
	panic("goev: IOHandle OnClose")
}
