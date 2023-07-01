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

type EvHandler interface {
	// Call by acceptor on `accept` a new fd or connector on `connect` successful
	//
	// Call OnClose() when return false
	OnOpen(r *Reactor, fd *Fd) bool

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

type NullEvent struct{}

func (n *NullEvent) OnOpen(r *Reactor, fd *Fd) bool {
	panic("NullEvent OnOpen")
	return false
}
func (n *NullEvent) OnRead(fd *Fd) bool {
	panic("NullEvent OnRead")
	return false
}
func (n *NullEvent) OnWrite(fd *Fd) bool {
	panic("NullEvent OnWrite")
	return false
}
func (n *NullEvent) OnConnectFail(err error) {
	panic("NullEvent OnConnectFail")
}
func (n *NullEvent) OnTimeout(now time.Time) bool {
	panic("NullEvent OnTimeout")
	return false
}
func (n *NullEvent) OnClose(fd *Fd) {
	panic("NullEvent OnClose")
}
