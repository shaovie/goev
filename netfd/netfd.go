package netfd

import (
	"errors"
	"net"
	"strconv"
	"syscall"
)

// On success, the number of bytes read is returned (zero indicates socket closed)
// On error, -1 is returned, and err is set appropriately
func Read(fd int, buf []byte) (n int, err error) {
	for {
		n, err = syscall.Read(fd, buf)
		if err != nil && err == syscall.EINTR {
			continue
		}
		break
	}
	return
}

func Write(fd int, buf []byte) (n int, err error) {
	for {
		n, err = syscall.Write(fd, buf)
		if err != nil && err == syscall.EINTR {
			continue
		}
		break
	}
	return
}

func Close(fd int) error {
	return syscall.Close(fd)
}

// Return format 192.168.0.1:8080
// Return "", if error
func LocalAddr(fd int) string {
	sa, _ := syscall.Getsockname(fd)
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
func RemoteAddr(fd int) string {
	sa, _ := syscall.Getpeername(fd)
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
func SetSendBuffSize(fd, bytes int) error {
	if err := syscall.SetsockoptInt(fd, syscall.SOL_SOCKET, syscall.SO_SNDBUF, bytes); err != nil {
		return errors.New("Set SO_SNDBUF: " + err.Error())
	}
	return nil
}

// 0:delay, 1:nodelay
func SetNoDelay(fd, v int) error {
	if err := syscall.SetsockoptInt(fd, syscall.IPPROTO_TCP, syscall.TCP_NODELAY, v); err != nil {
		return errors.New("Set TCP_NODELAY: " + err.Error())
	}
	return nil
}

// The all params are in second
//
// idle: After establishing a connection, if there is no data transmission during the "idle" time, a keep-alive packet will be sent
// interval: The interval period after the start of probing
// times: If there is no response after "times" attempts, the connection will be closed.
func SetKeepAlive(fd, idle, interval, times int) error {
	if interval < 1 {
		return errors.New("keepalive interval invalid")
	}
	if err := syscall.SetsockoptInt(fd, syscall.SOL_SOCKET, syscall.SO_KEEPALIVE, 1); err != nil {
		return errors.New("Set SO_KEEPALIVE: " + err.Error())
	}
	if err := syscall.SetsockoptInt(fd, syscall.IPPROTO_TCP, syscall.TCP_KEEPIDLE, idle); err != nil {
		return errors.New("Set TCP_KEEPIDLE: " + err.Error())
	}
	if err := syscall.SetsockoptInt(fd, syscall.IPPROTO_TCP, syscall.TCP_KEEPINTVL, interval); err != nil {
		return errors.New("Set TCP_KEEPINTVL: " + err.Error())
	}
	if err := syscall.SetsockoptInt(fd, syscall.IPPROTO_TCP, syscall.TCP_KEEPCNT, times); err != nil {
		return errors.New("Set TCP_KEEPCNT: " + err.Error())
	}
	return nil
}

// 0:delay 1:quick
func SetQuickACK(fd, bytes int) error {
	if err := syscall.SetsockoptInt(fd, syscall.IPPROTO_TCP, syscall.TCP_QUICKACK, 1); err != nil {
		return errors.New("Set TCP_QUICKACK: " + err.Error())
	}
	return nil
}
