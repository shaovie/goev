package goev

import (
	"errors"
	"net"
	"os"
	"strconv"
	"strings"
	"syscall"

	"golang.org/x/sys/unix"
)

// Acceptor is a wrapper for socket listener, automatically creating a service
// and registering it with the reactor.
// Newly received file descriptors can be registered with the specified reactor.
type Acceptor struct {
	Event

	reuseAddr        bool // SO_REUSEADDR
	reusePort        bool // SO_REUSEPORT
	fd               int
	sockRcvBufSize   int // ignore equal 0
	listenBacklog    int
	loopAcceptTimes  int
	newEvHanlderFunc func() EvHandler
	reactor          *Reactor
	newFdBindReactor *Reactor
}

// NewAcceptor return an acceptor
//
// New socket has been set to non-blocking
func NewAcceptor(acceptorBindReactor *Reactor, newFdBindReactor *Reactor,
	newEvHanlderFunc func() EvHandler, addr string, opts ...Option) (*Acceptor, error) {
	evOptions := setOptions(opts...)
	a := &Acceptor{
		fd:               -1,
		reactor:          acceptorBindReactor,
		newFdBindReactor: newFdBindReactor,
		newEvHanlderFunc: newEvHanlderFunc,
		listenBacklog:    evOptions.listenBacklog,
		sockRcvBufSize:   evOptions.sockRcvBufSize,
		reuseAddr:        evOptions.reuseAddr,
		reusePort:        evOptions.reusePort,
	}
	a.loopAcceptTimes = a.listenBacklog / 2
	if a.loopAcceptTimes < 1 {
		a.loopAcceptTimes = 1
	}
	if err := a.open(addr); err != nil {
		return nil, err
	}
	return a, nil
}

// open create a listen fd
// The addr format 192.168.0.1:8080 or :8080 or unix:/tmp/xxxx.sock
func (a *Acceptor) open(addr string) error {
	p := strings.Index(addr, ":")
	if p < 0 || p >= (len(addr)-1) {
		return errors.New("Accetor open param:addr invalid")
	}
	if len(addr) > 5 {
		s := addr[0:5]
		if s == "unix:" {
			return a.udsListen(addr[5:])
		}
	}
	return a.tcpListen(addr)
}

// The addr format 192.168.0.1:8080 or :8080
func (a *Acceptor) tcpListen(addr string) error {
	fd, err := syscall.Socket(syscall.AF_INET, syscall.SOCK_STREAM, 0)
	if err != nil {
		return errors.New("Socket in Acceptor.open: " + err.Error())
	}

	if a.reuseAddr == true {
		if err = syscall.SetsockoptInt(fd, syscall.SOL_SOCKET, syscall.SO_REUSEADDR, 1); err != nil {
			syscall.Close(fd)
			return errors.New("Set SO_REUSEADDR in Acceptor.open: " + err.Error())
		}
	}
	if a.reusePort == true {
		if err = syscall.SetsockoptInt(fd, syscall.SOL_SOCKET, unix.SO_REUSEPORT, 1); err != nil {
			syscall.Close(fd)
			return errors.New("Set SO_REUSEPORT in Acceptor.open: " + err.Error())
		}
	}
	syscall.SetNonblock(fd, true)

	if a.sockRcvBufSize > 0 {
		// `sysctl -a | grep net.ipv4.tcp_rmem` 返回 min default max
		// 默认 内核会在 min max 之间动态调整, default是初始值, 如果设置了SO_RCVBUF, 缓冲区大小不变成固定值,
		// 内核也不会进行动态调整了
		// 必须在listen/connect之前调用
		// must < `sysctl -a | grep net.core.rmem_max`
		err = syscall.SetsockoptInt(fd, syscall.SOL_SOCKET, syscall.SO_RCVBUF, a.sockRcvBufSize)
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

	if err := a.listen(fd, &sa); err != nil {
		syscall.Close(fd)
		return err
	}
	return nil
}

// The addr format /tmp/xxx.sock
func (a *Acceptor) udsListen(addr string) error {
	os.RemoveAll(addr)

	fd, err := syscall.Socket(syscall.AF_UNIX, syscall.SOCK_STREAM, 0)
	if err != nil {
		return errors.New("Socket in Acceptor.open: " + err.Error())
	}

	syscall.SetNonblock(fd, true)

	// SO_RCVBUF is invalid for unix sock

	rsu := syscall.SockaddrUnix{Name: addr}
	if err = a.listen(fd, &rsu); err != nil {
		os.RemoveAll(addr)
		syscall.Close(fd)
		return err
	}
	return nil
}

func (a *Acceptor) listen(fd int, sa syscall.Sockaddr) error {
	if err := syscall.Bind(fd, sa); err != nil {
		return errors.New("syscall bind: " + err.Error())
	}
	if err := syscall.Listen(fd, a.listenBacklog); err != nil {
		return errors.New("syscall listen: " + err.Error())
	}

	if err := a.reactor.AddEvHandler(a, fd, EvAccept); err != nil {
		return errors.New("AddEvHandler in Acceptor.Open: " + err.Error())
	}
	a.fd = fd
	return nil
}

// OnRead handle listner accept event
func (a *Acceptor) OnRead(fd int, rw IOReadWriter, now int64) bool {
	for i := 0; i < a.loopAcceptTimes; i++ {
		conn, _, err := syscall.Accept4(a.fd, syscall.SOCK_NONBLOCK|syscall.SOCK_CLOEXEC)
		if err != nil {
			if err == syscall.EINTR {
				continue
			}
			break
		}
		h := a.newEvHanlderFunc()
		h.setReactor(a.newFdBindReactor)
		if h.OnOpen(conn, now) == false {
			h.OnClose(conn)
		}
	}
	return true
}

// OnClose will not happen
func (a *Acceptor) OnClose(fd int) {
	syscall.Close(fd)
}
