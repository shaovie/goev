package goev

import (
	"net"
	"errors"
	"strconv"
	"strings"
	"syscall"
)

type acceptor struct {
	NullEvHandler

	fd               int
	events           uint32
	loopAcceptTimes  int
	newEvHanlderFunc func() EvHandler
	reactor          *Reactor
}

func NewAcceptor(opts ...Option) (*acceptor, error) {
	setOptions(opts...)
	a := &acceptor{
		fd: -1,
	}
	return a, nil
}

// Open create a listen fd
// The addr format 192.168.0.1:8080 or :8080
// The events list are in ev_handler.go
func (a *acceptor) Open(r *Reactor, newEvHanlderFunc func() EvHandler, addr string, events uint32) error {
	fd, err := syscall.Socket(syscall.AF_INET, syscall.SOCK_STREAM, 0)
	if err != nil {
		return errors.New("Socket in acceptor.Open: " + err.Error())
	}

	if evOptions.reuseAddr == true {
		if err = syscall.SetsockoptInt(fd, syscall.SOL_SOCKET, syscall.SO_REUSEADDR, 1); err != nil {
			syscall.Close(fd)
			return errors.New("Set SO_REUSEADDR in acceptor.Open: " + err.Error())
		}
	}
	syscall.SetNonblock(fd, true)

	if evOptions.recvBuffSize > 0 {
		// `sysctl -a | grep net.ipv4.tcp_rmem` 返回 min default max
		// 默认 内核会在 min max 之间动态调整, default是初始值, 如果设置了SO_RCVBUF, 缓冲区大小不变成固定值,
		// 内核也不会进行动态调整了
		// 必须在listen/connect之前调用
		// must < `sysctl -a | grep net.core.rmem_max`
		err = syscall.SetsockoptInt(fd, syscall.SOL_SOCKET, syscall.SO_RCVBUF, evOptions.recvBuffSize)
		if err != nil {
			syscall.Close(fd)
			return errors.New("Set SO_RCVBUF: " + err.Error())
		}
	}

	ip := "0.0.0.0"
	var port int64
	ipp := strings.Split(addr, ":")
	if len(ipp) != 2 {
		syscall.Close(fd)
		return errors.New("address is invalid! 192.168.1.1:80 or :80")
	}
	if len(ipp[0]) > 0 {
		ip = ipp[0]
	}
	ip4 := net.ParseIP(ip)
	if ip4 == nil {
		syscall.Close(fd)
		return errors.New("address is invalid! 192.168.1.1:80 or :80")
	}
	port, _ = strconv.ParseInt(ipp[1], 10, 64)
	if port < 1 || port > 65535 {
		syscall.Close(fd)
		return errors.New("port must in (0, 65536)")
	}
	sa := syscall.SockaddrInet4{Port: int(port)}
	copy(sa.Addr[:], ip4.To4())
	if err = syscall.Bind(fd, &sa); err != nil {
		syscall.Close(fd)
		return errors.New("syscall bind: " + err.Error())
	}
	if err = syscall.Listen(fd, evOptions.listenBacklog); err != nil {
		syscall.Close(fd)
		return errors.New("syscall listen: " + err.Error())
	}

	if err = r.AddEvHandler(a, fd, EV_ACCEPT); err != nil {
		syscall.Close(fd)
		return errors.New("AddEvHandler in acceptor.Open: " + err.Error())
	}
	a.reactor = r
	a.fd = fd
	a.events = events
	a.newEvHanlderFunc = newEvHanlderFunc
	a.loopAcceptTimes = evOptions.listenBacklog / 2
	if a.loopAcceptTimes < 1 {
		a.loopAcceptTimes = 1
	}
	return nil
}
func (a *acceptor) OnRead(fd *Fd) bool {
	for i := 0; i < a.loopAcceptTimes; i++ {
		conn, _, err := syscall.Accept4(a.fd, syscall.SOCK_NONBLOCK|syscall.SOCK_CLOEXEC)
		if err != nil {
			// if !errors.Is(err, syscall.EINTR) {
			//     return false
			// }
			break
		}
		h := a.newEvHanlderFunc()
		newFd := Fd{v: conn}
		if err = a.reactor.AddEvHandler(h, conn, a.events); err != nil {
			h.OnClose(&newFd)
			continue
		}
		if h.OnOpen(&newFd) == false {
			h.OnClose(&newFd)
			continue
		}
	}
	return true
}
func (a *acceptor) OnClose(fd *Fd) {
	// TODO close and reopen ?
}
