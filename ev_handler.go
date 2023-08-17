package goev

import (
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

// EvHandler is the event handling interface of the Reactor core
//
// The same EvHandler is repeatedly registered with the Reactor
type EvHandler interface {
	setFd(fd int)
	setParams(fd int, ep *evPoll)
	getEvPoll() *evPoll

	setReactor(r *Reactor)
	GetReactor() *Reactor

	setTimerItem(ti *timerItem)
	getTimerItem() *timerItem

	// Fd return fd
	Fd() int

	// OnOpen call by acceptor on `accept` a new fd or connector on `connect` successful
	//
	// Call OnClose() when return false
	OnOpen() bool

	// OnRead evpoll catch readable i/o event
	//
	// Call OnClose() when return false
	OnRead() bool

	// OnWrite evpoll catch writeable i/o event
	//
	// Call OnClose() when return false
	OnWrite() bool

	// OnConnectFail evpoll catch connect result
	// Only be asynchronously called after connector.Connect() returns nil
	//
	// Will not call OnClose() after OnConnectFail() (So you don't need to manually release the fd)
	// The param err Refer to ev_handler.go: ErrConnect*
	OnConnectFail(err error)

	// OnTimeout evpoll catch timeout event
	// The parameter 'millisecond' represents the time of batch retrieval of epoll events, not the current
	// precise time. Use it with caution (as it can reduce the frequency of obtaining the current
	// time to some extent).
	//
	// Remove timer when return false
	OnTimeout(millisecond int64) bool

	// OnClose call by reactor(OnOpen must have been called before calling OnClose.)
	//
	// You need to manually release the fd resource call fd.Close()
	// You'd better only call fd.Close() here.
	OnClose()

	// Write
	Write(bf []byte) (int, error)

	//= Async I/O
	// AsyncWrite submit data to async send queue
	//
	// The data will be synchronously sent by evpoll within its coroutine.
	// Each bf ensures ordered processing according to the sequence received by AsynWrite.
	// Whenever a bf is processed (regardless of the send result), the OnAsyncWriteBufDone method is called.
	// The framework strives to ensure timely data transmission and maintains order (
	// data that fails to send will be stored in a separate queue and prioritized for the next
	// transmission to ensure bf order).

	// NOTE: Each bf invokes a syscall.Write once. The framework does not perform secondary assembly
	// on bf (if needed, please assemble it manually)
	AsyncWrite(eh EvHandler, buf []byte)
	asyncOrderedWrite(ev EvHandler, abf asyncWriteBuf)

	// OnAsyncWriteBufDone callback after bf used (within the evpoll coroutine),
	// you can recycle bf. If no recycling is needed, you can ignore this method (Ignored in IOHandle).
	//
	// flag is passed by AsyncWrite{Flag}
	//OnAsyncWriteBufDone(bf []byte, flag int)

	// Destroy If you are using the Async write mechanism, it is essential to call the Destroy method
	// in OnClose to clean up any unsent bf data.
	// The cleanup process will also invoke OnAsyncWriteBufDone
	Destroy(eh EvHandler)
}

// Detecting illegal struct copies using `go vet`
type noCopy struct{}

func (*noCopy) Lock()   {}
func (*noCopy) Unlock() {}
