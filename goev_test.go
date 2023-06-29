package goev

import (
	"errors"
	"sync"
	"syscall"
	"testing"
)

var (
	buffPool *sync.Pool
)

func Test_Http(t *testing.T) {
	HttpServ(t)
}

const httpResp = "HTTP/1.1 200 OK\r\nConnection: keep-alive\r\nContent-Length: 5\r\n\r\nhello"

type Conn struct {
	NullEvHandler
}

func (c *Conn) OnOpen(fd *Fd) bool {
	return true
}
func (c *Conn) OnRead(fd *Fd) bool {
	for {
		buf := buffPool.Get().([]byte) // just read
		n, err := fd.Read(buf)
		buffPool.Put(buf)
		if err != nil {
			if errors.Is(err, syscall.EAGAIN) { // epoll ET mode
				break
			}
			return false
		}
		if n == 0 { // connection closed
			return false
		}
	}
	fd.Write([]byte(httpResp))
	return false // will goto OnClose
}
func (c *Conn) OnClose(fd *Fd) {
	fd.Close()
}

func HttpServ(t *testing.T) {
	buffPool = &sync.Pool{
		New: func() any {
			return make([]byte, 4096)
		},
	}
	r, err := NewReactor(
		EvPollSize(1024),
		EvPollThreadNum(2),
	)
	if err != nil {
		panic(err.Error())
	}
	if err = r.Open(); err != nil {
		panic(err.Error())
	}
	acceptor, err := NewAcceptor(
		ListenBacklog(256),
		RecvBuffSize(8*1024), // 短链接, 不需要很大的缓冲区
	)
	if err != nil {
		panic(err.Error())
	}
	acceptor.Open(r, func() EvHandler {
		return new(Conn)
	},
		":2023",
		EV_IN,
	)

	if err = r.Run(); err != nil {
		panic(err.Error())
	}
}
