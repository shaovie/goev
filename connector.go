package goev

import (
	"errors"
	"net"
	"strconv"
	"strings"
	"syscall"
)

var (
	// ErrConnectFail means connection failure without a specific reason
	ErrConnectFail = errors.New("connect fail")

	// ErrConnectTimeout means connection timeout
	ErrConnectTimeout = errors.New("connect timeout")

	// ErrConnectInprogress means the process is ongoing and not immediately successful.
	ErrConnectInprogress = errors.New("connect EINPROGRESS")
)

// Connector provides a fast asynchronous connector and can set a timeout.
// It internally uses Reactor to achieve asynchronicity.
// Connect success or failure will trigger specified methods for notification
type Connector struct {
	Event

	sockRcvBufSize int // ignore equal 0
}

// NewConnector return an instance
func NewConnector(r *Reactor, opts ...Option) (*Connector, error) {
	evOptions := setOptions(opts...)
	c := &Connector{
		sockRcvBufSize: evOptions.sockRcvBufSize,
	}
	c.setReactor(r)
	return c, nil
}

// Connect asynchronously to the specified address and there may also be an immediate result.
// Please check the return value
//
// The addr format 192.168.0.1:8080 or unix:/tmp/xxxx.sock
// The domain name format, such as qq.com:8080, is not supported.
// You need to manually extract the IP address using gethostbyname.
//
// Timeout is relative time measurements with millisecond accuracy, for example, delay=5msec.
func (c *Connector) Connect(addr string, eh EvHandler, timeout int64) error {
	if timeout < 0 {
		return errors.New("Connector:Connect param:timeout < 0")
	}
	p := strings.Index(addr, ":")
	if p < 0 || p >= (len(addr)-1) {
		return errors.New("Connector:Connect param:addr invalid")
	}
	if len(addr) > 5 {
		s := addr[0:5]
		if s == "unix:" {
			return c.udsConnect(addr[5:], eh, timeout)
		}
	}
	return c.tcpConnect(addr, eh, timeout)
}

// The addr format 192.168.0.1:8080
func (c *Connector) tcpConnect(addr string, eh EvHandler, timeout int64) error {
	fd, err := syscall.Socket(syscall.AF_INET,
		syscall.SOCK_STREAM|syscall.SOCK_NONBLOCK|syscall.SOCK_CLOEXEC, 0)
	if err != nil {
		return errors.New("Socket in connector.open: " + err.Error())
	}

	if c.sockRcvBufSize > 0 {
		// `sysctl -a | grep net.ipv4.tcp_rmem` 返回 min default max
		// 默认 内核会在 min max 之间动态调整, default是初始值, 如果设置了SO_RCVBUF, 缓冲区大小不变成固定值,
		// 内核也不会进行动态调整了
		// 必须在listen/connect之前调用
		// must < `sysctl -a | grep net.core.rmem_max`
		err = syscall.SetsockoptInt(fd, syscall.SOL_SOCKET, syscall.SO_RCVBUF, c.sockRcvBufSize)
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
	return c.connect(fd, &sa, eh, timeout)
}

func (c *Connector) udsConnect(addr string, eh EvHandler, timeout int64) error {
	fd, err := syscall.Socket(syscall.AF_UNIX,
		syscall.SOCK_STREAM|syscall.SOCK_NONBLOCK|syscall.SOCK_CLOEXEC, 0)
	if err != nil {
		return errors.New("Socket in connector.open: " + err.Error())
	}
	// SO_RCVBUF is invalid for unix sock
	rsu := syscall.SockaddrUnix{Name: addr}
	return c.connect(fd, &rsu, eh, timeout)
}

func (c *Connector) connect(fd int, sa syscall.Sockaddr, eh EvHandler, timeout int64) (err error) {
	reactor := c.GetReactor()
	for {
		err = syscall.Connect(fd, sa)
		if err == syscall.EINTR {
			continue
		}
		break
	}
	if err == syscall.EINPROGRESS {
		if timeout < 1 {
			return ErrConnectInprogress
		}
		inh := &inProgressConnect{eh: eh, fd: fd}
		if err = reactor.AddEvHandler(inh, fd, EvConnect); err != nil {
			syscall.Close(fd)
			return errors.New("InPorgress AddEvHandler in connector.Connect: " + err.Error())
		}
		inh.ScheduleTimer(inh, timeout, 0) // don't need to cancel it when conn error
		return nil
	} else if err == nil { // success
		eh.setReactor(reactor)
		if eh.OnOpen(fd) == false {
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

	fd int
	eh EvHandler
}

// Called by reactor when asynchronous connections fail.
func (p *inProgressConnect) OnRead(fd int) bool {
	p.eh.OnConnectFail(ErrConnectFail)
	return false // goto p.OnClose()
}

// Called by reactor when asynchronous connections succeed.
func (p *inProgressConnect) OnWrite(fd int) bool {
	// From here on, the `fd` resources will be managed by h.
	p.GetReactor().RemoveEvHandler(p, fd)
	p.fd = -1 //

	p.eh.setReactor(p.GetReactor())
	if p.eh.OnOpen(fd) == false {
		p.eh.OnClose(fd)
	}
	return true
}

// Called if a connection times out before completing.
func (p *inProgressConnect) OnTimeout(now int64) bool {
	// i/o event not catched
	p.eh.OnConnectFail(ErrConnectTimeout)
	p.OnClose(p.fd)
	return false
}

func (p *inProgressConnect) OnClose(fd int) {
	if p.fd != -1 {
		syscall.Close(p.fd)
		p.fd = -1
	}
}
