package goev

import (
	"net"
	"errors"
	"strconv"
	"syscall"
	//"sync/atomic"
)

var (
    EAGAIN = errors.New("EAGAIN")
)

// Fd 不能由外边构造
type Fd struct {
	v int
    ed *evData // internal var `for modify'

    // 防止fd重复被close(如果被close一次, fd数值有可能被OS复用给新的链接)
    // Prevent fd from being closed multiple times (if closed once,
    // the fd value may be reused by the OS for a new connection).
    // atomic.Int32 closed
}

func (fd *Fd) Fd() int {
    return fd.v
}
// On  success, the number of bytes read is returned (zero indicates socket closed)
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
func (fd *Fd) Close() {
    // if !fd.closed.CompareAndSwap(0, 1) {
    //     return
    // }
	syscall.Close(fd.v)
	fd.v = -1
}

// Return format 192.168.0.1:8080
// Return "", if error
func (fd *Fd) GetLocalAddr() string {
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
func (fd *Fd) GetPeerAddr() string {
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

// 0:delay 1:quick
func (fd *Fd) SetQuickACK(bytes int) error {
	if err := syscall.SetsockoptInt(fd.v, syscall.IPPROTO_TCP, syscall.TCP_QUICKACK, 1); err != nil {
		return errors.New("Set TCP_QUICKACK: " + err.Error())
	}
	return nil
}
