package goev

import (
	"errors"
	"net"
	"strconv"
	"strings"
	"sync/atomic"
	"syscall"
	"time"
)

var (
	ErrConnectFail       = errors.New("connect fail")
	ErrConnectTimeout    = errors.New("connect timeout")
	ErrConnectInprogress = errors.New("connect EINPROGRESS")
)

type Connector struct {
	Event

	recvBuffSize int // ignore equal 0
}

func NewConnector(r *Reactor, opts ...Option) (*Connector, error) {
	setOptions(opts...)
	c := &Connector{
		recvBuffSize: evOptions.recvBuffSize,
	}
	c.setReactor(r)
	return c, nil
}

// The addr format 192.168.0.1:8080
// The domain name format, such as qq.com:8080, is not supported.
// You need to manually extract the IP address using gethostbyname.
//
// Timeout is relative time measurements with millisecond accuracy, for example, delay=5msec.
func (c *Connector) Connect(addr string, eh EvHandler, timeout int64) error {
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
	reactor := c.GetReactor()
	for {
		err = syscall.Connect(fd, &sa)
		if err == syscall.EINTR {
			continue
		}
		break
	}
	if err == syscall.EINPROGRESS {
		if timeout < 1 {
			return ErrConnectInprogress
		}
		inh := &inProgressConnect{r: reactor, eh: eh, fd: fd}
		if err = reactor.AddEvHandler(inh, fd, EV_CONNECT); err != nil {
			syscall.Close(fd)
			return errors.New("InPorgress AddEvHandler in connector.Connect: " + err.Error())
		}
		reactor.SchedueTimer(inh, timeout, 0)
		return nil
	} else if err == nil { // success
		eh.setReactor(reactor)
		if eh.OnOpen(fd, time.Now().UnixMilli()) == false {
			eh.OnClose(fd)
		}
		return nil
	}
	syscall.Close(fd)
	return errors.New("syscall connect: " + err.Error())
}

// nonblocking inprogress connection
type inProgressConnect struct {
	Event

	fd           int
	eh           EvHandler
	r            *Reactor
	progressDone atomic.Int32 // Only process one I/O event or timer event
}

// Called by reactor when asynchronous connections fail.
func (p *inProgressConnect) OnRead(fd int, now int64) bool {
	if !p.progressDone.CompareAndSwap(0, 1) {
		return true
	}
	p.eh.OnConnectFail(ErrConnectFail)
	return false // goto p.OnClose()
}

// Called by reactor when asynchronous connections succeed.
func (p *inProgressConnect) OnWrite(fd int, now int64) bool {
	if !p.progressDone.CompareAndSwap(0, 1) {
		return true
	}
	// From here on, the `fd` resources will be managed by h.
	p.r.RemoveEvHandler(p, fd)
	p.fd = -1 //
	p.eh.setReactor(p.r)
	if p.eh.OnOpen(fd, now) == false {
		p.eh.OnClose(fd)
	}
	return true
}

// Called if a connection times out before completing.
func (p *inProgressConnect) OnTimeout(now int64) bool {
	if !p.progressDone.CompareAndSwap(0, 1) {
		return true
	}

	// i/o event not catched
	p.eh.OnConnectFail(ErrConnectTimeout)
	return false
}
func (p *inProgressConnect) OnClose(fd int) {
	if p.fd != -1 {
		syscall.Close(p.fd)
		p.fd = -1
	}
}
