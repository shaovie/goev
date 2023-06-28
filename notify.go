package goev

import (
	"errors"
	"syscall"
	"unsafe"

	"golang.org/x/sys/unix"
)

type Notify struct {
	NullEvHandler
	efd       int
	evHandler EvHandler
	reactor   *Reactor
}

var notifyCounterV int64 = 1
var notifyCounterWriteV = (*(*[8]byte)(unsafe.Pointer(&notifyCounterV)))[:]

func (nt *Notify) Open(r *Reactor, h EvHandler) error {
	if r == nil || h == nil {
		panic("Notify.Open args are nil")
	}
	// since Linux 2.6.27
	fd, err := unix.Eventfd(0, unix.EFD_NONBLOCK|unix.EFD_CLOEXEC)
	if err != nil {
		return errors.New("eventfd: " + err.Error())
	}
	nt.reactor = r
	nt.efd = fd
	nt.evHandler = h
	if err = nt.reactor.AddEvHandler(h, nt.efd, EV_EVENTFD); err != nil {
		syscall.Close(fd)
		return errors.New("Notify add to evpoll fail! " + err.Error())
	}
	return nil
}
func (nt *Notify) Close() {
	nt.reactor.RemoveEvHandler(nt.efd)
	syscall.Close(nt.efd)
	nt.efd = -1
}
func (nt *Notify) Notify() {
	// man 2 eventfd
	syscall.Write(nt.efd, notifyCounterWriteV)
}
