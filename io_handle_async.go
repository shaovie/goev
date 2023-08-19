package goev

import (
	"syscall"
)

func ioAllocBuff(s int) []byte {
	if ioBuffUseMemPool {
		return Malloc(s)
	}
	return make([]byte, s)
}
func ioFreeBuff(bf []byte) {
	if ioBuffUseMemPool {
		Free(bf)
	}
}

type asyncWriteBuf struct {
	writen int    // wrote len
	len    int    // buf original len. readonly
	buf    []byte // readonly
}

// AsyncOrderedFlush only called in OnWrite
//
// For example:
//
//	func (x *XX) OnWrite(fd int) {
//	    x.AsyncOrderedFlush(x)
//	}
func (h *IOHandle) AsyncOrderedFlush(eh EvHandler) {
	fd := h.Fd()
	if fd < 1 {
		return
	}
	n := h._asyncWriteBufQ.Len()
	// It is necessary to use n to limit the number of sending attempts.
	// If there is a possibility of sending failure, the data should be saved again in _asyncWriteBufQ
	for i := 0; i < n; i++ {
		abf, ok := h._asyncWriteBufQ.PopFront()
		if !ok {
			break
		}
		n, _ := syscall.Write(fd, abf.buf[abf.writen:abf.len])
		if n > 0 {
			h._asyncWriteBufSize -= n
			if n == (abf.len - abf.writen) { // send completely
				ioFreeBuff(abf.buf)
				continue
			}
			abf.writen += n // Partially write, shift n
		}
		h._asyncWriteBufQ.PushFront(abf)
		break
	}
	if h._asyncWriteBufQ.IsEmpty() {
		h._ep.remove(fd, EvOut)
		h._asyncWriteWaiting = false
	}
}

// AsyncWrite asynchronous write
//
// It is safe for concurrent use by multiple goroutines
func (h *IOHandle) AsyncWrite(eh EvHandler, buf []byte) {
	fd := h.Fd()
	if fd < 1 { // NOTE fd must > 0
		return
	}
	abf := ioAllocBuff(len(buf))
	n := copy(abf, buf)
	h._ep.push(asyncWriteItem{
		fd: fd,
		eh: eh,
		abf: asyncWriteBuf{
			len: n,
			buf: abf,
		},
	})
}

func (h *IOHandle) asyncOrderedWrite(eh EvHandler, abf asyncWriteBuf) {
	fd := h.Fd()
	if fd < 1 { // closed or except
		ioFreeBuff(abf.buf)
		return
	}
	h._asyncWriteBufSize += abf.len
	if h._asyncWriteBufQ != nil && !h._asyncWriteBufQ.IsEmpty() {
		h._asyncWriteBufQ.PushBack(abf)
		return
	}

	n, _ := syscall.Write(fd, abf.buf[abf.writen:abf.len])
	if n > 0 {
		h._asyncWriteBufSize -= n
		if n == (abf.len - abf.writen) {
			ioFreeBuff(abf.buf)
			return
		}
		abf.writen += n // Partially write, shift n
	}

	// Error or Partially
	if h._asyncWriteBufQ == nil {
		h._asyncWriteBufQ = NewRingBuffer[asyncWriteBuf](4)
	}
	h._asyncWriteBufQ.PushBack(abf)

	if h._asyncWriteWaiting == false {
		h._asyncWriteWaiting = true
		h._ep.append(fd, EvOut) // No need to use ET mode
		// eh needs to implement the OnWrite method, and the OnWrite method
		// needs to call AsyncOrderedFlush.
	}
}

// AsyncWaitWriteQLen The length of the queue waiting to be sent asynchronously
//
// If it is too long, it indicates that the sending is slow and the receiving end is abnormal
func (h *IOHandle) AsyncWaitWriteQLen() int {
	if h._asyncWriteBufQ == nil {
		return 0
	}
	return h._asyncWriteBufQ.Len()
}
