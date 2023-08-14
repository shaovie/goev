package goev

import (
	"syscall"
)

type asyncWriteBuf struct {
	// allowing you to know the actual processing progress.
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
				continue
			}
			abf.writen += n // Partially write, shift n
		}
		h._asyncWriteBufQ.PushFront(abf)
		break
	}
	if h._asyncWriteBufQ.IsEmpty() {
		h._ep.subtract(fd, EvOut)
		h._asyncWriteWaiting = false
	}
}

// AsyncWrite asynchronous write
func (h *IOHandle) AsyncWrite(eh EvHandler, buf []byte) {
	fd := h.Fd()
	if fd > 0 { // NOTE fd must > 0
		abf := make([]byte, len(buf)) // TODO optimize
		copy(abf, buf)
		h._ep.push(asyncWriteItem{
			fd: fd,
			eh: eh,
			abf: asyncWriteBuf{
				len: len(buf),
				buf: abf,
			},
		})
	}
}

func (h *IOHandle) asyncOrderedWrite(eh EvHandler, abf asyncWriteBuf) {
	fd := h.Fd()
	if fd < 1 { // closed or except
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
			return
		}
		abf.writen += n // Partially write, shift n
	}

	// Error or Partially
	if h._asyncWriteBufQ == nil {
		h._asyncWriteBufQ = NewRingBuffer[asyncWriteBuf](2)
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
