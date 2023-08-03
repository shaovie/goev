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
	Read(fd int) ([]byte, error)
	ReadWouldBlock(fd int) ([]byte, error)

	InitWrite()

	Append(v []byte)

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
	wlen       int
	buf        []byte
}

// NewIOReadWriter return an instance
func NewIOReadWriter(bufSize, maxBufSize int) IOReadWriter {
	return &IOReadWrite{
		maxBufSize: maxBufSize,
		buf:        make([]byte, bufSize),
	}
}
func (rw *IOReadWrite) growBuf() bool {
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

// InitWrite init iowrite buf
func (rw *IOReadWrite) InitWrite() {
	rw.buf = rw.buf[:]
	rw.wlen = 0
	rw.closed = false
}

// Append fill write buf
func (rw *IOReadWrite) Append(v []byte) {
	vl := len(v)
	if vl > (cap(rw.buf) - rw.wlen) {
		if rw.growBuf() == false {
			panic("sockio buf exceeds the limit")
		}
	}
	copy(rw.buf[rw.wlen:], v)
	rw.wlen += vl
}

// Closed return true if connection closed
func (rw *IOReadWrite) Closed() bool {
	return rw.closed
}

// Read attempt to read a buffer sized data block once
func (rw *IOReadWrite) Read(fd int) ([]byte, error) {
	rw.buf = rw.buf[:]
	rw.closed = false
	readN, n := 0, 0
	var err error
	for {
		n, err = syscall.Read(fd, rw.buf[readN:])
		if n > 0 {
			readN += n
			if readN == cap(rw.buf) {
				if rw.growBuf() == false {
					err = ErrRcvBufOutOfLimit
					break
				}
			}
			break
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

// ReadWouldBlock will attempt to receive as much readable data as possible,
// directly until syscall.Read returns an error (excluding EINTER)
//
// For EPOLLET mode
func (rw *IOReadWrite) ReadWouldBlock(fd int) ([]byte, error) {
	rw.buf = rw.buf[:]
	rw.closed = false
	readN, n := 0, 0
	var err error
	for {
		n, err = syscall.Read(fd, rw.buf[readN:])
		if n > 0 {
			readN += n
			if readN == cap(rw.buf) {
				if rw.growBuf() == false {
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
	writeN := 0
	for writeN < rw.wlen {
		n, err = syscall.Write(fd, rw.buf[writeN:rw.wlen])
		if n > 0 {
			writeN += n
			if writeN == rw.wlen {
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
