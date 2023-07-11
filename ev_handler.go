package goev

import (
	"syscall"
)

const (
	EPOLLET          = 1 << 31
	EV_IN     uint32 = syscall.EPOLLIN | syscall.EPOLLRDHUP
	EV_OUT    uint32 = syscall.EPOLLOUT | syscall.EPOLLRDHUP
	EV_IN_ET  uint32 = EV_IN | EPOLLET
	EV_OUT_ET uint32 = EV_OUT | EPOLLET

	EV_EVENTFD uint32 = syscall.EPOLLIN | syscall.EPOLLRDHUP // Not ET mode

	// 用水平触发, 循环Accept有可能会导致不可控
	EV_ACCEPT uint32 = syscall.EPOLLIN | syscall.EPOLLRDHUP

	EV_CONNECT uint32 = syscall.EPOLLIN | syscall.EPOLLOUT | syscall.EPOLLRDHUP
)

// Detecting illegal struct copies using `go vet`
type noCopy struct{}

func (*noCopy) Lock()   {}
func (*noCopy) Unlock() {}

// The same EvHandler is repeatedly registered with the Reactor
type EvHandler interface {
	setEvPoll(ep *evPoll)
	getEvPoll() *evPoll

	setReactor(r *Reactor)
	GetReactor() *Reactor

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
	OnRead(fd int, evPollSharedBuff []byte, millisecond int64) bool

	// EvPoll catch writeable i/o event
	// The parameter 'millisecond' represents the time of batch retrieval of epoll events, not the current
	// precise time. Use it with caution (as it can reduce the frequency of obtaining the current
	// time to some extent).
	//
	// Call OnClose() when return false
	OnWrite(fd int, millisecond int64) bool

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

type Event struct {
	noCopy

	_r *Reactor // atomic.Pointer[Reactor]
	// 这里不需要保护, 在set之前Get是没有任何调用机会的(除非框架之外乱搞)

	_ep *evPoll // atomic.Pointer[evPoll]
	// 这里不需要保护, 在set之前Get是没有任何调用机会的(除非框架之外乱搞)
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
func (e *Event) GetReactor() *Reactor {
	return e._r
}

func (*Event) OnOpen(fd int, millisecond int64) bool {
	panic("Event OnOpen")
	return false
}
func (*Event) OnRead(fd int, evPollSharedBuff []byte, millisecond int64) bool {
	panic("Event OnRead")
	return false
}
func (*Event) OnWrite(fd int, millisecond int64) bool {
	panic("Event OnWrite")
	return false
}
func (*Event) OnConnectFail(err error) {
	panic("Event OnConnectFail")
}
func (*Event) OnTimeout(millisecond int64) bool {
	panic("Event OnTimeout")
	return false
}
func (*Event) OnClose(fd int) {
	panic("Event OnClose")
}
