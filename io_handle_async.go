package goev

import (
	"syscall"
	"time"
)

// AsyncWriteBuf x
type AsyncWriteBuf struct {
	Flag int // The flag will be returned in OnAsyncWriteBufDone,
	// allowing you to know the actual processing progress.
	Writen int    // wrote len
	Len    int    // buf original len. readonly
	Buf    []byte // readonly
}

// AsyncWrite asynchronous write
func (h *IOHandle) AsyncWrite(eh EvHandler, abf AsyncWriteBuf) {
	if h._fd > 0 { // NOTE fd must > 0
		h._ep.push(asyncWriteItem{
			fd:  h._fd,
			eh:  eh,
			abf: abf,
		})
	} else {
		eh.OnAsyncWriteBufDone(abf.Buf, abf.Flag)
	}
}

func (h *IOHandle) asyncOrderedWrite(eh EvHandler, abf AsyncWriteBuf) {
	if h._fd < 1 { // closed or except
		eh.OnAsyncWriteBufDone(abf.Buf, abf.Flag)
		return
	}
	if h._asyncWriteBufQ != nil && !h._asyncWriteBufQ.IsEmpty() {
		h._asyncWriteBufQ.Push(abf)
		return
	}

	if abf.Len < 1 || abf.Writen >= abf.Len {
		eh.OnAsyncWriteBufDone(abf.Buf, abf.Flag)
		return
	}
	n, _ := syscall.Write(h._fd, abf.Buf[abf.Writen:abf.Len])
	if n > 0 {
		if n == (abf.Len - abf.Writen) {
			h._asyncLastPartialWriteTime = 0
			eh.OnAsyncWriteBufDone(abf.Buf, abf.Flag) // send completely
			return
		}
		abf.Writen += n // Partially write, shift n
	}

	h._asyncLastPartialWriteTime = time.Now().UnixMilli()
	// Error or Partially
	if h._asyncWriteBufQ == nil {
		h._asyncWriteBufQ = NewRingBuffer[AsyncWriteBuf](2)
	}
	h._asyncWriteBufQ.Push(abf)

	if h._asyncWriteWaiting == false {
		h._asyncWriteWaiting = true
		h._ep.append(h._fd, EvOut) // No need to use ET mode
		// eh needs to implement the OnWrite method, and the OnWrite method needs to call AsyncOrderedFlush.
	}
}

// AsyncOrderedFlush only called in OnWrite
//
// For example:
//
//	func (x *XX) OnWrite(fd int) {
//	    x.AsyncOrderedFlush(x)
//	}
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
		abf, ok := h._asyncWriteBufQ.Pop()
		if !ok {
			break
		}
		eh.asyncOrderedWrite(eh, abf)
	}
	if h._asyncWriteBufQ.IsEmpty() {
		h._ep.subtract(h._fd, EvOut)
		h._asyncWriteWaiting = false
	}
}

// OnAsyncWriteBufDone callback after bf used (within the evpoll coroutine),
func (h *IOHandle) OnAsyncWriteBufDone(bf []byte, flag int) {
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

// AsyncLastPartialWriteTime indicates that the previous write was incomplete and requires 'evpoll'
// to polling for the writable state. This value helps prevent a connection from being indefinitely
// unreachable due to abnormalities or the recipient not receiving data. Millisecond
//
// e.g.
// if nowMilli - x.AsyncLastPartialWriteTime() > 10*1000 && x.AsyncWaitWriteQLen() > 0 { // 10secs
//
//	    x.GetReactor().RemoveEvHandler(x, x.Fd())
//	    x.OnClose(x.Fd())
//	    return // The connection lifecycle has ended
//	}
func (h *IOHandle) AsyncLastPartialWriteTime() int64 {
	return h._asyncLastPartialWriteTime
}
