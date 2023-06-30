package goev

import (
	"errors"
	"net"
	"strconv"
	"syscall"
)

// Fd 不能由外边构造
type Fd struct {
	v int
    ed *evData // internal var `for modify'
}

func (fd *Fd) Fd() int {
    return fd.v
}
func (fd *Fd) Read(buf []byte) (int, error) {
	return syscall.Read(fd.v, buf)
}
func (fd *Fd) Write(buf []byte) (int, error) {
	return syscall.Write(fd.v, buf)
}
func (fd *Fd) Close() {
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
