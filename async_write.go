package goev

import (
	"errors"
	"sync"
	"sync/atomic"
	"syscall"
	"unsafe"

	"golang.org/x/sys/unix"
)

// Asynchronously written data block.
// The framework will ensure that it is sent out in the order in which it was enqueued
type asyncWriteItem struct {
	fd  int
	eh  EvHandler
	abf asyncWriteBuf
}

// Using a double buffer queue, the 'writeq' is only responsible for receiving data blocks.
// When it is time to send, the 'writeq' and 'readq' are swapped,
// and the 'readq' (previously the 'writeq') is responsible for sending
type asyncWrite struct {
	IOHandle

	efd      int
	notified atomic.Int32 // used to avoid duplicate call evHandler

	readq  *RingBuffer[asyncWriteItem]
	writeq *RingBuffer[asyncWriteItem]
	mtx    sync.Mutex

	evPoll *evPoll
}

func newAsyncWrite(ep *evPoll) (*asyncWrite, error) {
	a := &asyncWrite{
		readq:  NewRingBuffer[asyncWriteItem](256),
		writeq: NewRingBuffer[asyncWriteItem](256),
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
func (aw *asyncWrite) push(awi asyncWriteItem) {
	aw.mtx.Lock()
	aw.writeq.PushBack(awi)
	aw.mtx.Unlock()

	if !aw.notified.CompareAndSwap(0, 1) {
		return
	}
	var v int64 = 1
	for {
		_, err := syscall.Write(aw.efd, (*(*[8]byte)(unsafe.Pointer(&v)))[:]) // man 2 eventfd
		if err != nil && err == syscall.EINTR {
			continue
		}
		break
	}
}

// OnRead writeq has data
func (aw *asyncWrite) OnRead() bool {
	if aw.readq.IsEmpty() {
		aw.mtx.Lock()
		aw.writeq, aw.readq = aw.readq, aw.writeq // Swap read/write queues
		aw.mtx.Unlock()
	}

	for i := 0; i < 256; i++ { // Don't process too many at once
		item, ok := aw.readq.PopFront()
		if !ok {
			break
		}
		ed := aw.evPoll.loadEvData(item.fd)
		if ed != nil && ed.eh == item.eh { // TODO Comparing interfaces, the performance is not very good
			item.eh.asyncOrderedWrite(item.eh, item.abf)
		} else {
			ioFreeBuff(item.abf.buf)
		}
	}

	if !aw.readq.IsEmpty() { // Ignore readable eventfd, continue
		return true
	}

	var bf [8]byte
	for {
		_, err := syscall.Read(aw.efd, bf[:])
		if err != nil {
			if err == syscall.EINTR {
				continue
			} else if err == syscall.EAGAIN {
				return true
			}
			return false // TODO add evOptions.debug? panic("Notify: read eventfd failed!")
		}
		aw.notified.Store(0)
		break
	}
	return true
}
