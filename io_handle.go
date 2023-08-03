package goev

import (
	"errors"
)

// IOHandle is the base class of io event handling objects
type IOHandle struct {
	noCopy

	_r *Reactor

	_ep *evPoll

	_ti *timerItem
}

// Init IOHandle must be called when reusing it.
func (h *IOHandle) Init() {
	h._r, h._ep, h._ti = nil, nil, nil
}

func (h *IOHandle) setEvPoll(ep *evPoll) {
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

// ScheduleTimer Add a timer event to an IOHandle that is already registered with the reactor
// to ensure that all event handling occurs within the same evpoll
//
// Only supports binding timers to I/O objects within evpoll internally.
func (h *IOHandle) ScheduleTimer(eh EvHandler, delay, interval int64) error {
	if ep := eh.getEvPoll(); ep != nil {
		return ep.scheduleTimer(eh, delay, interval)
	}
	return errors.New("ev handler has not been added to the reactor yet")
}

// CancelTimer cancels a timer that has been successfully scheduled
func (h *IOHandle) CancelTimer(eh EvHandler) {
	if ep := eh.getEvPoll(); ep != nil {
		ep.cancelTimer(eh)
	}
}

// Read use evPollReadBuff, buf size can set by options.EvPollReadBuffSize
func (h *IOHandle) Read(fd int) (bf []byte, n int, err error) {
	if ep := h.getEvPoll(); ep != nil {
		bf, n, err = ep.read(fd)
	} else {
		panic("goev: IOHandle.Read fd not register to evpoll")
	}
	return
}

// WriteBuff must be registered with evpoll in order to be used
func (h *IOHandle) WriteBuff() []byte {
	if ep := h.getEvPoll(); ep != nil {
		return ep.writeBuff()
	}
	panic("goev: IOHandle.WriteBuff fd not register to evpoll")
}

// OnOpen please make sure you want to reimplement it.
func (*IOHandle) OnOpen(fd int) bool {
	panic("goev: IOHandle OnOpen")
}

// OnRead please make sure you want to reimplement it.
func (*IOHandle) OnRead(fd int) bool {
	panic("goev: IOHandle OnRead")
}

// OnWrite please make sure you want to reimplement it.
func (*IOHandle) OnWrite(fd int) bool {
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
func (*IOHandle) OnClose(fd int) {
	panic("goev: IOHandle OnClose")
}
