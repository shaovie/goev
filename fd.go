package goev

import (
	"errors"
	"net"
	"strconv"
	"sync/atomic"
	"syscall"
)

type Fd struct {
	noCopy
	v int

	// 防止fd重复被close(如果被close, fd数值有可能被OS复用给新的链接, 再次调用就会出问题)
	// 也是对Fd.v的一种保护
	// Prevent fd from being closed multiple times (if closed once,
	// the fd value may be reused by the OS for a new connection).
	// It's also a form of concurrent access protection for Fd.v
	closed atomic.Int32
}

func (fd *Fd) reset(v int) {
	fd.v = v
	fd.closed.Store(0) // Don't forget
}

// Get file descriptor
func (fd *Fd) Fd() int {
	return fd.v
}

// io.Reader
// On success, the number of bytes read is returned (zero indicates socket closed)
// On error, -1 is returned, and err is set appropriately
func (fd *Fd) Read(buf []byte) (n int, err error) {
	for {
		n, err = syscall.Read(fd.v, buf)
		if err != nil && err == syscall.EINTR {
			continue
		}
		break
	}
	return
}
// io.Writer
func (fd *Fd) Write(buf []byte) (n int, err error) {
	for {
		n, err = syscall.Write(fd.v, buf)
		if err != nil && err == syscall.EINTR {
			continue
		}
		break
	}
	return
}
// Just for io.Closer return 'error'
func (fd *Fd) Close() error {
	if !fd.closed.CompareAndSwap(0, 1) {
		return nil
	}
	syscall.Close(fd.v)
	fd.v = -1
    return nil
}

// Return format 192.168.0.1:8080
// Return "", if error
func (fd *Fd) LocalAddr() string {
	sa, _ := syscall.Getsockname(fd.v)
	ip := net.IP{}
	port := 0
	switch sa := sa.(type) {
	case *syscall.SockaddrInet4:
		copy([]byte(ip), sa.Addr[0:])
		port = sa.Port
	case *syscall.SockaddrInet6:
		copy([]byte(ip), sa.Addr[0:])
		port = sa.Port
	default:
		return ""
	}
	return ip.String() + ":" + strconv.FormatInt(int64(port), 10)
}

// Return format 192.168.0.1:8080
// Return "", if error
func (fd *Fd) RemoteAddr() string {
	sa, _ := syscall.Getpeername(fd.v)
	ip := net.IP{}
	port := 0
	switch sa := sa.(type) {
	case *syscall.SockaddrInet4:
		copy([]byte(ip), sa.Addr[0:])
		port = sa.Port
	case *syscall.SockaddrInet6:
		copy([]byte(ip), sa.Addr[0:])
		port = sa.Port
	default:
		return ""
	}
	return ip.String() + ":" + strconv.FormatInt(int64(port), 10)
}

// 在accept/connnect之后调用
// must < `sysctl -a | grep net.core.wmem_max`
func (fd *Fd) SetSendBuffSize(bytes int) error {
	if err := syscall.SetsockoptInt(fd.v, syscall.SOL_SOCKET, syscall.SO_SNDBUF, bytes); err != nil {
		return errors.New("Set SO_SNDBUF: " + err.Error())
	}
	return nil
}

// 0:delay, 1:nodelay
func (fd *Fd) SetNoDelay(v int) error {
	if err := syscall.SetsockoptInt(fd.v, syscall.IPPROTO_TCP, syscall.TCP_NODELAY, v); err != nil {
		return errors.New("Set TCP_NODELAY: " + err.Error())
	}
	return nil
}
// The all params are in second
//
// idle: After establishing a connection, if there is no data transmission during the "idle" time, a keep-alive packet will be sent
// interval: The interval period after the start of probing
// times: If there is no response after "times" attempts, the connection will be closed.
func (fd *Fd) SetKeepAlive(idle, interval, times int) error {
    if interval < 1 {
        return errors.New("keepalive interval invalid")
    }
	if err := syscall.SetsockoptInt(fd.v, syscall.SOL_SOCKET, syscall.SO_KEEPALIVE, 1); err != nil {
		return errors.New("Set SO_KEEPALIVE: " + err.Error())
	}
	if err := syscall.SetsockoptInt(fd.v, syscall.IPPROTO_TCP, syscall.TCP_KEEPIDLE, idle); err != nil {
		return errors.New("Set TCP_KEEPIDLE: " + err.Error())
	}
	if err := syscall.SetsockoptInt(fd.v, syscall.IPPROTO_TCP, syscall.TCP_KEEPINTVL, interval); err != nil {
		return errors.New("Set TCP_KEEPINTVL: " + err.Error())
	}
	if err := syscall.SetsockoptInt(fd.v, syscall.IPPROTO_TCP, syscall.TCP_KEEPCNT, times); err != nil {
		return errors.New("Set TCP_KEEPCNT: " + err.Error())
	}
	return nil
}

// 0:delay 1:quick
func (fd *Fd) SetQuickACK(bytes int) error {
	if err := syscall.SetsockoptInt(fd.v, syscall.IPPROTO_TCP, syscall.TCP_QUICKACK, 1); err != nil {
		return errors.New("Set TCP_QUICKACK: " + err.Error())
	}
	return nil
}
