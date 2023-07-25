package goev

import (
	"errors"
	"syscall"
)

var (
	// ErrRcvBufOutOfLimit means receive buffer exceeds the limit
	ErrRcvBufOutOfLimit = errors.New("receive buffer exceeds the limit")
)

// IOReadWriter is a lock-free, shared-buffer IO processor for each evpoll
// execution stack cycle (without considering thread contention).
type IOReadWriter interface {
	InitRead() IOReadWriter
	InitWrite() IOReadWriter

	Append(v []byte) IOReadWriter

	Read(fd int) ([]byte, error)

	Write(fd int) (n int, err error)

	Closed() bool
}

// IOReadWrite default implement
//
// Reading data can increase the buffer capacity, but writing data does not impose any restrictions
// on the buffer capacity
type IOReadWrite struct {
	closed bool

	maxBufSize int
	buf        []byte
}

// NewIOReadWriter return an instance
func NewIOReadWriter(bufSize, maxBufSize int) IOReadWriter {
	return &IOReadWrite{
		maxBufSize: maxBufSize,
		buf:        make([]byte, bufSize),
	}
}
func (rw *IOReadWrite) growReadBuf() bool {
	c := cap(rw.buf)
	if c == rw.maxBufSize {
		return false
	}
	newSize := c + c/2
	if newSize > rw.maxBufSize {
		newSize = rw.maxBufSize
	}
	newBuf := make([]byte, newSize)
	copy(newBuf, rw.buf)
	rw.buf = newBuf
	return true
}

// InitRead init ioread buf
func (rw *IOReadWrite) InitRead() IOReadWriter {
	rw.buf = rw.buf[:]
	rw.closed = false
	return rw
}

// InitWrite init iowrite buf
func (rw *IOReadWrite) InitWrite() IOReadWriter {
	rw.buf = rw.buf[:0]
	rw.closed = false
	return rw
}

// Append fill write buf
func (rw *IOReadWrite) Append(v []byte) IOReadWriter {
	rw.buf = append(rw.buf, v...)
	return rw
}

// Closed return true if connection closed
func (rw *IOReadWrite) Closed() bool {
	return rw.closed
}

// Read will attempt to receive as much readable data as possible,
// directly until syscall.Read returns an error (excluding EINTER)
func (rw *IOReadWrite) Read(fd int) ([]byte, error) {
	readN, n := 0, 0
	var err error
	for {
		n, err = syscall.Read(fd, rw.buf[readN:])
		if n > 0 {
			readN += n
			if readN == cap(rw.buf) {
				if rw.growReadBuf() == false {
					err = ErrRcvBufOutOfLimit
					break
				}
			}
			continue
		} else if n == 0 { // peer closed
			rw.closed = true
			break
		} else {
			if err != nil {
				if err == syscall.EINTR {
					continue
				}
				break // incluce EAGAIN
			}
		}
	}
	return rw.buf[:readN], err
}

// Write before using the Write method, use the Append method to add the data to be sent.
// It will attempt to send as much as possible, but there is no guarantee of complete transmission
func (rw *IOReadWrite) Write(fd int) (n int, err error) {
	wlen := len(rw.buf)
	writeN := 0
	for writeN < wlen {
		n, err = syscall.Write(fd, rw.buf[writeN:])
		if n > 0 {
			writeN += n
			if writeN == wlen {
				break
			}
			continue
		} else if n == 0 { // peer closed
			rw.closed = true
			break
		} else {
			if err != nil {
				if err == syscall.EINTR {
					continue
				} else if err == syscall.EPIPE {
					rw.closed = true
				}
				break // incluce EAGAIN
			}
		}
	}
	n = writeN
	return
}
