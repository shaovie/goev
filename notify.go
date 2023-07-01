package goev

import (
	"errors"
	"unsafe"
	"syscall"
	"sync/atomic"

	"golang.org/x/sys/unix"
)

type Notifier interface {
    // Tread-safe
    Notify()

    // Tread-safe close the notifier
    Close()
}

type NotifyHandler func()

type Notify struct {
	NullEvent

	efd        int
    notifyOnce atomic.Int32 // used to avoid duplicate call evHandler
    closeOnce  atomic.Int32 // used to avoid duplicate close

	handler NotifyHandler
	reactor   *Reactor
}
var (
    notifyV int64 = 1
    notifyWriteV = (*(*[8]byte)(unsafe.Pointer(&notifyV)))[:]
    notifyCloseV int64 = 31415927
    notifyCloseWriteV = (*(*[8]byte)(unsafe.Pointer(&notifyCloseV)))[:]
)

func NewNotify(r *Reactor, h NotifyHandler) (Notifier, error) {
	if r == nil || h == nil {
		return nil, errors.New("NewNotify invalid params")
	}
	// since Linux 2.6.27
	fd, err := unix.Eventfd(0, unix.EFD_NONBLOCK|unix.EFD_CLOEXEC)
	if err != nil {
		return nil, errors.New("eventfd: " + err.Error())
	}
    nt := &Notify{
        reactor: r,
        efd: fd,
        handler: h,
    }
	if err = nt.reactor.AddEvHandler(nt, nt.efd, EV_EVENTFD); err != nil {
		syscall.Close(fd)
		return nil, errors.New("Notify add to evpoll fail! " + err.Error())
	}
	return nt, nil
}
func (nt *Notify) Notify() {
    if !nt.notifyOnce.CompareAndSwap(0, 1) {
        return
    }
    for {
        n, err := syscall.Write(nt.efd, notifyWriteV) // man 2 eventfd
        if n == 8 {
            return
        } else if err != nil {
            if err == syscall.EINTR {
                continue
            }
            if err == syscall.EAGAIN {
                return
            }
        }
        break // TODO add evOptions.debug? panic("Notify: write eventfd failed!")
    }
}
func (nt *Notify) Close() {
    if !nt.closeOnce.CompareAndSwap(0, 1) {
        return
    }
    for {
        n, err := syscall.Write(nt.efd, notifyCloseWriteV) // man 2 eventfd
        if n == 8 {
            return
        }
        if err != nil {
            if err == syscall.EINTR {
                continue
            }
            // err == syscall.EAGAIN
        }
        nt.notifyOnce.Store(0)
        break // TODO add evOptions.debug? panic("Notify: write eventfd failed!")
    }
}

// Prohibit external calls
func (nt *Notify) OnRead(fd *Fd) bool {
    if fd.v != nt.efd { // 防止外部调用!
        panic("Prohibit external calls")
    }
    var tmp[8]byte
    for {
        n, err := syscall.Read(nt.efd, tmp[:])
        if err != nil {
            if err == syscall.EINTR {
                continue
            }
            if err == syscall.EAGAIN {
                nt.notifyOnce.Store(0)
                return true
            }
            return false // TODO add evOptions.debug? panic("Notify: read eventfd failed!")
        }
        if n == 8 {
            if *(*int64)(unsafe.Pointer(&tmp[0])) == notifyV {
                nt.notifyOnce.Store(0)
                nt.handler()
                return true
            }
            if *(*int64)(unsafe.Pointer(&tmp[0])) == notifyCloseV {
                nt.closeOnce.Store(0) // optional
                return false // goto OnClose
            }
            return false // TODO add evOptions.debug? panic("Notify: read unknown value!")
        }
    }
    return true // 
}
func (nt *Notify) OnClose(fd *Fd) {
    fd.Close()
	nt.efd = -1
}
