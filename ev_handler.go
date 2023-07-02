package goev

import (
	"time"
	"syscall"
)

const (
    EPOLLET = 1 << 31
	EV_IN  uint32 = syscall.EPOLLIN | EPOLLET | syscall.EPOLLRDHUP
	EV_OUT uint32 = syscall.EPOLLOUT | EPOLLET | syscall.EPOLLRDHUP

	EV_EVENTFD uint32 = syscall.EPOLLIN | syscall.EPOLLRDHUP // Not ET mode

	// 用水平触发, 循环Accept有可能会导致不可控
	EV_ACCEPT uint32 = syscall.EPOLLIN | syscall.EPOLLRDHUP

	EV_CONNECT uint32 = syscall.EPOLLIN | syscall.EPOLLOUT | syscall.EPOLLRDHUP
)

// Detecting illegal struct copies using `go vet`
type noCopy struct {}
func (*noCopy) Lock() {}
func (*noCopy) Unlock() {}

type EvHandler interface {
    setReactor(r *Reactor)
    GetReactor() *Reactor

	// Call by acceptor on `accept` a new fd or connector on `connect` successful
	//
	// Call OnClose() when return false
	OnOpen(fd *Fd) bool

	// EvPoll catch readable i/o event
	//
	// Call OnClose() when return false
	OnRead(fd *Fd) bool

	// EvPoll catch writeable i/o event
	//
	// Call OnClose() when return false
	OnWrite(fd *Fd) bool

	// EvPoll catch connect result
    // Only be asynchronously called after connector.Connect() returns nil
	//
	// Don't call OnClose() after OnConnectFail() be called
	OnConnectFail(err error)

	// EvPoll catch timeout event
	// TODO
	// Call OnClose() when return false
	OnTimeout(now time.Time) bool

    // Call by reactor(OnOpen must have been called before calling OnClose.)
    //
	// You need to manually release the fd resource call fd.Close()
	// You'd better only call fd.Close() here.
	OnClose(fd *Fd)
}

type Event struct{
     noCopy
     _r *Reactor // atomic.Pointer[Reactor]
                // 这里不需要保护, 在set之前Get是没有任何调用机会的(除非框架之外乱搞)
}

func (e *Event) setReactor(r *Reactor) {
    e._r = r
}
func (e *Event) GetReactor() *Reactor {
    return e._r
}
func (*Event) OnOpen(fd *Fd) bool {
	panic("Event OnOpen")
	return false
}
func (*Event) OnRead(fd *Fd) bool {
	panic("Event OnRead")
	return false
}
func (*Event) OnWrite(fd *Fd) bool {
	panic("Event OnWrite")
	return false
}
func (*Event) OnConnectFail(err error) {
	panic("Event OnConnectFail")
}
func (*Event) OnTimeout(now time.Time) bool {
	panic("Event OnTimeout")
	return false
}
func (*Event) OnClose(fd *Fd) {
	panic("Event OnClose")
}
