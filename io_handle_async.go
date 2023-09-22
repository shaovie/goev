package goev

import (
	"syscall"
)

func ioAllocBuff(s int) []byte {
	if ioBuffUsePool {
		return BMalloc(s)
	}
	return make([]byte, s)
}
func ioFreeBuff(bf []byte) {
	if ioBuffUsePool {
		BFree(bf)
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
	n := h.asyncWriteBufQ.Len()
	// It is necessary to use n to limit the number of sending attempts.
	// If there is a possibility of sending failure, the data should be saved again in _asyncWriteBufQ
	for i := 0; i < n; i++ {
		abf, ok := h.asyncWriteBufQ.PopFront()
		if !ok {
			break
		}
		n, _ := syscall.Write(fd, abf.buf[abf.writen:abf.len])
		if n > 0 {
			h.asyncWriteBufSize -= n
			if n == (abf.len - abf.writen) { // send completely
				ioFreeBuff(abf.buf)
				continue
			}
			abf.writen += n // Partially write, shift n
		}
		h.asyncWriteBufQ.PushFront(abf)
		break
	}
	if h.asyncWriteBufQ.IsEmpty() {
		h.ep.remove(fd, EvOut)
		h.asyncWriteWaiting = false
		h.OnWriteBufferDrained()
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
	if fd != eh.Fd() { // Ensure that it is the same object
		panic("goev: AsyncWrite EvHandler is invalid")
	}
	abf := ioAllocBuff(len(buf))
	n := copy(abf, buf) // if n != len(buf) panic ?
	h.ep.push(asyncWriteItem{
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
	h.asyncWriteBufSize += abf.len
	if h.asyncWriteBufQ != nil && !h.asyncWriteBufQ.IsEmpty() {
		h.asyncWriteBufQ.PushBack(abf)
		return
	}

	n, _ := syscall.Write(fd, abf.buf[abf.writen:abf.len])
	if n > 0 {
		h.asyncWriteBufSize -= n
		if n == (abf.len - abf.writen) {
			ioFreeBuff(abf.buf)
			return
		}
		abf.writen += n // Partially write, shift n
	}

	// Error or Partially
	if h.asyncWriteBufQ == nil {
		h.asyncWriteBufQ = NewRingBuffer[asyncWriteBuf](4)
	}
	h.asyncWriteBufQ.PushBack(abf)

	if h.asyncWriteWaiting == false {
		h.asyncWriteWaiting = true
		h.ep.append(fd, EvOut) // No need to use ET mode
		// eh needs to implement the OnWrite method, and the OnWrite method
		// needs to call AsyncOrderedFlush.
	}
}

// AsyncWaitWriteQLen The length of the queue waiting to be sent asynchronously
//
// If it is too long, it indicates that the sending is slow and the receiving end is abnormal
func (h *IOHandle) AsyncWaitWriteQLen() int {
	if h.asyncWriteBufQ == nil {
		return 0
	}
	return h.asyncWriteBufQ.Len()
}
