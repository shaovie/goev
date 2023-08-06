package goev

import (
	"syscall"
)

//
//= Asynchronous write

// AsyncWrite asynchronous write
func (h *IOHandle) AsyncWrite(eh EvHandler, bf []byte) {
	if h._fd > 0 { // NOTE fd must > 0
		h._ep.push(asyncWriteItem{
			fd: h._fd,
			eh: eh,
			bf: bf,
		})
	}
}

func (h *IOHandle) asyncOrderedWrite(eh EvHandler, bf []byte, tryTimes int) {
	if h._fd < 1 { // closed or except
		eh.OnAsyncWriteBufDone(bf)
		return
	}
	if h._asyncWriteBufQ != nil && !h._asyncWriteBufQ.IsEmpty() {
		h._asyncWriteBufQ.Push(asyncPartialWriteBuf{
			len:      0,
			tryTimes: tryTimes + 1,
			bf:       bf,
		})
		return
	}

	writen := 0
	n, _ := syscall.Write(h._fd, bf[writen:])
	if n > 0 {
		if n == len(bf) {
			eh.OnAsyncWriteBufDone(bf) // send ok
			return
		}
		writen = n // Partially write
	}

	// Error or Partially
	if h._asyncWriteBufQ == nil {
		h._asyncWriteBufQ = NewRingBuffer[asyncPartialWriteBuf](2)
	}
	h._asyncWriteBufQ.Push(asyncPartialWriteBuf{
		len: writen,
		bf:  bf,
	})
	h._ep.remove(h._fd)
	h._ep.append(h._fd, EvOut) // No need to use ET mode
	// eh needs to implement the OnWrite method, and the OnWrite method needs to call AsyncOrderedFlush.
	// For example:
	// func (x *XX) OnWrite(fd int) {
	//     x.AsyncOrderedFlush(x)
	// }
}

// AsyncOrderedFlush
func (h *IOHandle) AsyncOrderedFlush(eh EvHandler) {
	if h._asyncWriteBufQ == nil || h._asyncWriteBufQ.IsEmpty() {
		return
	}
	if h._fd < 1 {
		return
	}
	n := h._asyncWriteBufQ.Len()
	// It is necessary to use n to limit the number of sending attempts.
	// If there is a possibility of sending failure, the data should be saved again in _asyncWriteBufQ
	for i := 0; i < n; i++ {
		bf, ok := h._asyncWriteBufQ.Pop()
		if !ok {
			break
		}
		eh.asyncOrderedWrite(eh, bf.bf, bf.tryTimes)
	}
	if h._asyncWriteBufQ.IsEmpty() {
		h._ep.subtract(h._fd, EvOut)
	}
}

// OnAsyncWriteBufDone callback after bf used (within the evpoll coroutine),
func (h *IOHandle) OnAsyncWriteBufDone(bf []byte) {
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
