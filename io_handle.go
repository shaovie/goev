package goev

import (
	"errors"
	"sync/atomic"
	"syscall"
)

// IOHandle is the base class of io event handling objects
type IOHandle struct {
	noCopy

	_asyncWriteWaiting bool
	_fd                int32
	_asyncWriteBufSize int // total size of wait to write

	_r *Reactor

	_ep *evPoll

	_ti *timerItem

	_asyncWriteBufQ *RingBuffer[asyncWriteBuf] // 保存未直接发送完成的
}

// Init IOHandle must be called when reusing it.
func (h *IOHandle) Init() {
	h._r, h._ep, h._ti = nil, nil, nil
	h.setFd(-1)
}

func (h *IOHandle) setParams(fd int, ep *evPoll) {
	h.setFd(fd)
	h._ep = ep
}

func (h *IOHandle) getEvPoll() *evPoll {
	return h._ep
}

func (h *IOHandle) setReactor(r *Reactor) {
	h._r = r
}

// GetReactor can retrieve the current event object bound to which Reactor
func (h *IOHandle) GetReactor() *Reactor {
	return h._r
}

func (h *IOHandle) setTimerItem(ti *timerItem) {
	h._ti = ti
}

func (h *IOHandle) getTimerItem() *timerItem {
	return h._ti
}

// Fd return fd
func (h *IOHandle) Fd() int {
	return int(atomic.LoadInt32(&(h._fd)))
}
func (h *IOHandle) setFd(fd int) {
	atomic.StoreInt32(&(h._fd), int32(fd))
}

// ScheduleTimer Add a timer event to an IOHandle that is already registered with the reactor
// to ensure that all event handling occurs within the same evpoll
//
// Only supports binding timers to I/O objects within evpoll internally.
func (h *IOHandle) ScheduleTimer(eh EvHandler, delay, interval int64) error {
	if h._ep != nil {
		return h._ep.scheduleTimer(eh, delay, interval)
	}
	return errors.New("ev handler has not been added to the reactor yet")
}

// CancelTimer cancels a timer that has been successfully scheduled
func (h *IOHandle) CancelTimer(eh EvHandler) {
	if h._ep != nil {
		h._ep.cancelTimer(eh)
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
	if h._ep != nil {
		bf, n, err = h._ep.read(fd)
	} else {
		panic("goev: IOHandle.Read fd not register to evpoll")
	}
	return
}

// WriteBuff must be registered with evpoll in order to be used
//
// Can only be used within the poller goroutine
func (h *IOHandle) WriteBuff() []byte {
	if h._ep != nil {
		return h._ep.writeBuff()
	}
	panic("goev: IOHandle.WriteBuff fd not register to evpoll")
}

// Write synchronous write.
// n = [n, len(bf]
func (h *IOHandle) Write(bf []byte) (n int, err error) {
	fd := h.Fd()
	if fd < 1 { // NOTE fd must > 0
		return 0, syscall.EBADF
	}
	if h._asyncWriteBufQ != nil && !h._asyncWriteBufQ.IsEmpty() {
		abf := make([]byte, len(bf)) // TODO optimize
		n = copy(abf, bf)
		h._asyncWriteBufQ.PushBack(asyncWriteBuf{
			len: n,
			buf: abf,
		})
		h._asyncWriteBufSize += n
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
		abf := make([]byte, len(bf)-n) // TODO optimize
		n = copy(abf, bf[n:])
		if h._asyncWriteBufQ == nil {
			h._asyncWriteBufQ = NewRingBuffer[asyncWriteBuf](2)
		}
		h._asyncWriteBufQ.PushBack(asyncWriteBuf{
			len: n,
			buf: abf,
		})
		h._asyncWriteBufSize += n
		if h._asyncWriteWaiting == false {
			h._asyncWriteWaiting = true
			h._ep.append(fd, EvOut) // No need to use ET mode
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
	h.setFd(-1)

	if h._asyncWriteBufQ != nil && !h._asyncWriteBufQ.IsEmpty() {
		for {
			_, ok := h._asyncWriteBufQ.PopFront()
			if !ok {
				break
			}
		}
	}
}

//
//= EvHandler interface

// OnOpen please make sure you want to reimplement it.
func (*IOHandle) OnOpen(fd int) bool {
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
