package goev

import (
	"syscall"
)

const (
	EV_IN  = syscall.EPOLLIN | syscall.EPOLLET | syscall.EPOLLRDHUP
	EV_OUT = syscall.EPOLLOUT | syscall.EPOLLET | syscall.EPOLLRDHUP

	EV_EVENTFD = syscall.EPOLLIN | syscall.EPOLLRDHUP

	// 用水平触发, 循环Accept有可能会导致不可控
	EV_ACCEPT = syscall.EPOLLIN | syscall.EPOLLRDHUP
	//EV_CONNECT  = EV_IN | EV_OUT
)

type EvHandler interface {
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

	// EvPoll catch notify event
	//
	// Call OnClose() when return false
	OnNotify(fd *Fd) bool

	// You need to manually release the fd resource call fd.Close()
	// You'd better only call fd.Close() here.
	OnClose(fd *Fd)
}

type NullEvHandler struct{}

func (n *NullEvHandler) OnOpen(fd *Fd) bool {
    panic("NullEvHandler OnOpen")
	return false
}
func (n *NullEvHandler) OnRead(fd *Fd) bool {
    panic("NullEvHandler OnRead")
	return false
}
func (n *NullEvHandler) OnWrite(fd *Fd) bool {
    panic("NullEvHandler OnWrite")
	return false
}
func (n *NullEvHandler) OnNotify(fd *Fd) bool {
    panic("NullEvHandler OnNotify")
	return false
}
func (n *NullEvHandler) OnClose(fd *Fd) {
    panic("NullEvHandler OnClose")
}
