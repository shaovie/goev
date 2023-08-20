package goev

import (
	"errors"
	"sync"
	"sync/atomic"
	"syscall"
	"unsafe"

	"golang.org/x/sys/unix"
)

////// Operate type define

// PollSyncCache to sync cache in evPoll
const PollSyncCache int = 1

// PollSyncCacheOpt sync arg
type PollSyncCacheOpt struct {
	ID    int
	Value any
}

///////////////////////////////////////

type pollSyncOptArg struct {
	typ int
	arg any
}

type pollSyncOpt struct {
	IOHandle

	efd      int
	notified atomic.Int32 // used to avoid duplicate call evHandler

	readq  *RingBuffer[pollSyncOptArg]
	writeq *RingBuffer[pollSyncOptArg]
	mtx    sync.Mutex

	evPoll *evPoll
}

func newPollSyncOpt(ep *evPoll) (*pollSyncOpt, error) {
	a := &pollSyncOpt{
		readq:  NewRingBuffer[pollSyncOptArg](4),
		writeq: NewRingBuffer[pollSyncOptArg](4),
	}

	fd, err := unix.Eventfd(0, unix.EFD_NONBLOCK|unix.EFD_CLOEXEC)
	if err != nil {
		return nil, errors.New("goev: eventfd " + err.Error())
	}
	if err = ep.add(fd, EvEventfd, a); err != nil {
		syscall.Close(fd)
		return nil, errors.New("goev.asyncWrite add to evpoll fail! " + err.Error())
	}
	a.efd = fd
	a.evPoll = ep

	return a, nil
}
func (c *pollSyncOpt) push(typ int, val any) {
	c.mtx.Lock()
	c.writeq.PushBack(pollSyncOptArg{
		typ: typ,
		arg: val,
	})
	c.mtx.Unlock()

	if !c.notified.CompareAndSwap(0, 1) {
		return
	}
	var v int64 = 1
	for {
		_, err := syscall.Write(c.efd, (*(*[8]byte)(unsafe.Pointer(&v)))[:]) // man 2 eventfd
		if err != nil && err == syscall.EINTR {
			continue
		}
		break
	}
}

// OnRead writeq has data
func (c *pollSyncOpt) OnRead() bool {
	if c.readq.IsEmpty() {
		c.mtx.Lock()
		c.writeq, c.readq = c.readq, c.writeq // Swap read/write queues
		c.mtx.Unlock()
	}

	for i := 0; i < 8; i++ { // Don't process too many at once
		item, ok := c.readq.PopFront()
		if !ok {
			break
		}
		c.doSync(item)
	}

	if !c.readq.IsEmpty() { // Ignore readable eventfd, continue
		return true
	}

	var bf [8]byte
	for {
		_, err := syscall.Read(c.efd, bf[:])
		if err != nil {
			if err == syscall.EINTR {
				continue
			} else if err == syscall.EAGAIN {
				return true
			}
			return false // TODO add evOptions.debug? panic("Notify: read eventfd failed!")
		}
		c.notified.Store(0)
		break
	}
	return true
}
func (c *pollSyncOpt) init(typ int, val any) {
	c.doSync(pollSyncOptArg{
		typ: typ,
		arg: val,
	})
}
func (c *pollSyncOpt) doSync(op pollSyncOptArg) {
	if op.typ == PollSyncCache {
		c.evPoll.pCacheSet(op.arg.(PollSyncCacheOpt).ID, op.arg.(PollSyncCacheOpt).Value)
	}
}
