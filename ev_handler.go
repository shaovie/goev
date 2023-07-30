package goev

import (
	"errors"
	"syscall"
)

const (
	// EPOLLET Refer to sys/epoll.h
	EPOLLET = 1 << 31

	// EvIn is readable event
	EvIn uint32 = syscall.EPOLLIN | syscall.EPOLLRDHUP

	// EvOut is writeable event
	EvOut uint32 = syscall.EPOLLOUT | syscall.EPOLLRDHUP

	// EvInET is readable event in EPOLLET mode
	EvInET uint32 = EvIn | EPOLLET

	// EvOutET is readable event in EPOLLET mode
	EvOutET uint32 = EvOut | EPOLLET

	// EvEventfd used for eventfd
	EvEventfd uint32 = syscall.EPOLLIN | syscall.EPOLLRDHUP // Not ET mode

	// EvAccept used for acceptor
	// 用水平触发, 循环Accept有可能会导致不可控
	EvAccept uint32 = syscall.EPOLLIN | syscall.EPOLLRDHUP

	// EvConnect used for connector
	EvConnect uint32 = syscall.EPOLLIN | syscall.EPOLLOUT | syscall.EPOLLRDHUP
)

// Detecting illegal struct copies using `go vet`
type noCopy struct{}

func (*noCopy) Lock()   {}
func (*noCopy) Unlock() {}

// EvHandler is the event handling interface of the Reactor core
//
// The same EvHandler is repeatedly registered with the Reactor
type EvHandler interface {
	setEvPoll(ep *evPoll)
	getEvPoll() *evPoll

	setReactor(r *Reactor)
	GetReactor() *Reactor

	setTimerItem(ti *timerItem)
	getTimerItem() *timerItem

	// ScheduleTimer Add a timer event to an Event that is already registered with the reactor
	// to ensure that all event handling occurs within the same evpoll
	//
	// Only supports binding timers to I/O objects within evpoll internally.
	ScheduleTimer(delay, interval int64) error

	// CancelTimer cancels a timer that has been successfully scheduled
	CancelTimer()

	// Call by acceptor on `accept` a new fd or connector on `connect` successful
	// The parameter 'millisecond' represents the time of batch retrieval of epoll events, not the current
	// precise time. Use it with caution (as it can reduce the frequency of obtaining the current
	// time to some extent).
	//
	// Call OnClose() when return false
	OnOpen(fd int, millisecond int64) bool

	// EvPoll catch readable i/o event
	// The parameter 'millisecond' represents the time of batch retrieval of epoll events, not the current
	// precise time. Use it with caution (as it can reduce the frequency of obtaining the current
	// time to some extent).
	//
	// Call OnClose() when return false
	OnRead(fd int, nio IOReadWriter, millisecond int64) bool

	// EvPoll catch writeable i/o event
	// The parameter 'millisecond' represents the time of batch retrieval of epoll events, not the current
	// precise time. Use it with caution (as it can reduce the frequency of obtaining the current
	// time to some extent).
	//
	// Call OnClose() when return false
	OnWrite(fd int, nio IOReadWriter, millisecond int64) bool

	// EvPoll catch connect result
	// Only be asynchronously called after connector.Connect() returns nil
	//
	// Will not call OnClose() after OnConnectFail() (So you don't need to manually release the fd)
	// The param err Refer to ev_handler.go: ErrConnect*
	OnConnectFail(err error)

	// EvPoll catch timeout event
	// The parameter 'millisecond' represents the time of batch retrieval of epoll events, not the current
	// precise time. Use it with caution (as it can reduce the frequency of obtaining the current
	// time to some extent).
	// Note: Don't call Reactor.SchedueTimer() or Reactor.CancelTimer() in OnTimeout, it will deadlock
	//
	// Remove timer when return false
	OnTimeout(millisecond int64) bool

	// Call by reactor(OnOpen must have been called before calling OnClose.)
	//
	// You need to manually release the fd resource call fd.Close()
	// You'd better only call fd.Close() here.
	OnClose(fd int)
}

// Event is the base class of event handling objects
type Event struct {
	noCopy

	_r *Reactor // atomic.Pointer[Reactor]
	// 这里不需要保护, 在set之前Get是没有任何调用机会的(除非框架之外乱搞)

	_ep *evPoll // atomic.Pointer[evPoll]
	// 这里不需要保护, 在set之前Get是没有任何调用机会的(除非框架之外乱搞)

	_ti *timerItem
}

// Init Event object must be called when reusing it.
func (e *Event) Init() {
	e._r, e._ep, e._ti = nil, nil, nil
}

func (e *Event) setEvPoll(ep *evPoll) {
	e._ep = ep
}

func (e *Event) getEvPoll() *evPoll {
	return e._ep
}

func (e *Event) setReactor(r *Reactor) {
	e._r = r
}

// GetReactor can retrieve the current event object bound to which Reactor
func (e *Event) GetReactor() *Reactor {
	return e._r
}

func (e *Event) setTimerItem(ti *timerItem) {
	e._ti = ti
}

func (e *Event) getTimerItem() *timerItem {
	return e._ti
}

// ScheduleTimer Add a timer event to an Event that is already registered with the reactor
// to ensure that all event handling occurs within the same evpoll
//
// Only supports binding timers to I/O objects within evpoll internally.
func (e *Event) ScheduleTimer(delay, interval int64) error {
	if ep := e.getEvPoll(); ep != nil {
		return ep.scheduleTimer(e, delay, interval)
	}
	return errors.New("ev handler has not been added to the reactor yet")
}

// CancelTimer cancels a timer that has been successfully scheduled
func (e *Event) CancelTimer() {
	if ep := e.getEvPoll(); ep != nil {
		ep.cancelTimer(e)
	}
}

// OnOpen please make sure you want to reimplement it.
func (*Event) OnOpen(fd int, millisecond int64) bool {
	panic("Event OnOpen")
}

// OnRead please make sure you want to reimplement it.
func (*Event) OnRead(fd int, nio IOReadWriter, millisecond int64) bool {
	panic("Event OnRead")
}

// OnWrite please make sure you want to reimplement it.
func (*Event) OnWrite(fd int, nio IOReadWriter, millisecond int64) bool {
	panic("Event OnWrite")
}

// OnConnectFail please make sure you want to reimplement it.
func (*Event) OnConnectFail(err error) {
	panic("Event OnConnectFail")
}

// OnTimeout please make sure you want to reimplement it.
func (*Event) OnTimeout(millisecond int64) bool {
	panic("Event OnTimeout")
}

// OnClose please make sure you want to reimplement it.
func (*Event) OnClose(fd int) {
	panic("Event OnClose")
}
