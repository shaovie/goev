package goev

import (
	"net"
	"time"
	"errors"
	"strings"
	"strconv"
	"syscall"
	"sync/atomic"
)

var (
    ErrConnectFail = errors.New("connect fail")
    ErrConnectTimeout = errors.New("connect timeout")
    ErrConnectAddEvHandler = errors.New("add handler to reactor error!")
)
type Connector struct {
	Event

	recvBuffSize     int  // ignore equal 0
	reactor          *Reactor
}

func NewConnector(r *Reactor, opts ...Option) (*Connector, error) {
	setOptions(opts...)
	c := &Connector{
        reactor: r,
        recvBuffSize: evOptions.recvBuffSize,
	}
	return c, nil
}

// The addr format 192.168.0.1:8080
// The domain name format, such as qq.com:8080, is not supported.
// You need to manually extract the IP address using gethostbyname.
func (c *Connector) Connect(addr string, h EvHandler, events uint32) error {
    panic("Not fully implemented") // TODO
	fd, err := syscall.Socket(syscall.AF_INET, syscall.SOCK_STREAM, 0)
	if err != nil {
		return errors.New("Socket in connector.open: " + err.Error())
	}

	syscall.SetNonblock(fd, true)

	if c.recvBuffSize > 0 {
		// `sysctl -a | grep net.ipv4.tcp_rmem` 返回 min default max
		// 默认 内核会在 min max 之间动态调整, default是初始值, 如果设置了SO_RCVBUF, 缓冲区大小不变成固定值,
		// 内核也不会进行动态调整了
		// 必须在listen/connect之前调用
		// must < `sysctl -a | grep net.core.rmem_max`
		err = syscall.SetsockoptInt(fd, syscall.SOL_SOCKET, syscall.SO_RCVBUF, c.recvBuffSize)
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
		return errors.New("address is invalid! 192.168.1.1:80")
	}
	if len(ipp[0]) > 0 {
		ip = ipp[0]
	}
	ip4 := net.ParseIP(ip)
	if ip4 == nil {
		syscall.Close(fd)
		return errors.New("address is invalid! 192.168.1.1:80")
	}
	port, _ = strconv.ParseInt(ipp[1], 10, 64)
	if port < 1 || port > 65535 {
		syscall.Close(fd)
		return errors.New("port must in (0, 65536)")
	}
	sa := syscall.SockaddrInet4{Port: int(port)}
	copy(sa.Addr[:], ip4.To4())
	err = syscall.Connect(fd, &sa)
    if err == nil { // success
		newFd := Fd{v: fd}
        h.setReactor(c.reactor)
		if h.OnOpen(&newFd, time.Now().UnixMilli()) == false {
			h.OnClose(&newFd)
		}
        if err = h.GetReactor().AddEvHandler(h, fd, events); err != nil {
            syscall.Close(fd) // not h.OnClose()
            return errors.New("AddEvHandler in connector.Connect: " + err.Error())
        }
        return nil
    } else if errors.Is(err, syscall.EINPROGRESS) {
        if err = c.reactor.AddEvHandler(&inProgressConnect{r:c.reactor, h:h, fd:fd, events: events},
            fd, EV_CONNECT); err != nil {
            syscall.Close(fd)
            return errors.New("InPorgress AddEvHandler in connector.Connect: " + err.Error())
        }
    } else {
		syscall.Close(fd)
		return errors.New("syscall connect: " + err.Error())
	}
	return nil
}
// nonblocking inprogress connection
type inProgressConnect struct {
    Event

    fd int
	events           uint32
    h EvHandler
    r *Reactor
    progressDone atomic.Int32 // Only process one I/O event or timer event
}
// Called by reactor when asynchronous connections fail. 
func (p *inProgressConnect) OnRead(fd *Fd, now int64) bool {
    if !p.progressDone.CompareAndSwap(0, 1) {
        return true
    }
    p.h.OnConnectFail(ErrConnectFail)
	return false // goto p.OnClose()
}
// Called by reactor when asynchronous connections succeed. 
func (p *inProgressConnect) OnWrite(fd *Fd, now int64) bool {
    if !p.progressDone.CompareAndSwap(0, 1) {
        return true
    }
    // From here on, the `fd` resources will be managed by h.
    p.h.setReactor(p.r)
    newFd := Fd{v: p.fd}
    if p.h.OnOpen(&newFd, now) == false {
        p.h.OnClose(&newFd)
    }
    if err := p.h.GetReactor().AddEvHandler(p.h, p.fd, p.events); err != nil {
        p.h.OnConnectFail(ErrConnectAddEvHandler)
        return false // goto p.OnClose()
    }
    // TODO cancel timer
	return true
}
// Called if a connection times out before completing.
func (p *inProgressConnect) OnTimeout(now int64) bool {
    if !p.progressDone.CompareAndSwap(0, 1) {
        return true
    }

    // i/o event not catched
    p.h.OnConnectFail(ErrConnectTimeout)
	return false
}
func (p *inProgressConnect) OnClose(fd *Fd) {
    fd.Close()
    // TODO cancel timer
}
