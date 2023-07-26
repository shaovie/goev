package main

import (
	"fmt"
	"sync"
	"syscall"

	"github.com/shaovie/goev"
	"github.com/shaovie/goev/netfd"
)

var (
	buffPool *sync.Pool
)

const httpResp = "HTTP/1.1 200 OK\r\nConnection: close\r\nContent-Length: 5\r\n\r\nhello"

type Http struct {
	goev.Event
	m [4096]byte // test memory leak
}

func (h *Http) OnOpen(fd int, now int64) bool {
	// fd.SetNoDelay(1) // New socket has been set to non-blocking
	if err := h.GetReactor().AddEvHandler(h, fd, goev.EvIn); err != nil {
		return false
	}
	return true
}
func (h *Http) OnRead(fd int, nio IOReadWriter, now int64) bool {
	_, err := nio.InitRead().Read(fd)
	if nio.Closed() || err == goev.ErrRcvBufOutOfLimit { // Abnormal connection
		return false
	}
	netfd.Write(fd, []byte(httpResp)) // Connection: close
	return false                      // will goto OnClose
}
func (h *Http) OnClose(fd int) {
	netfd.Close(fd)
}

type Https struct {
	Http
}

func main() {
	fmt.Println("hello boy")
	buffPool = &sync.Pool{
		New: func() any {
			return make([]byte, 4096)
		},
	}
	forAccept, err := goev.NewReactor(
		goev.EvDataArrSize(0), // default val
		goev.EvPollNum(1),
		goev.EvReadyNum(8), // only accept fd
	)
	if err != nil {
		panic(err.Error())
	}
	forNewFd, err := goev.NewReactor(
		goev.EvDataArrSize(0), // default val
		goev.EvPollNum(1),
		goev.EvReadyNum(512), // auto calc
	)
	if err != nil {
		panic(err.Error())
	}
	//= http
	_, err = goev.NewAcceptor(forAccept, forNewFd, func() goev.EvHandler { return new(Http) },
		":2023",
		goev.ListenBacklog(256),
		goev.SockRcvBufSize(8*1024), // 短链接, 不需要很大的缓冲区
	)
	if err != nil {
		panic(err.Error())
	}

	//= https
	_, err = goev.NewAcceptor(forAccept, forNewFd, func() goev.EvHandler { return new(Https) },
		":2024",
		goev.ListenBacklog(256),
		goev.SockRcvBufSize(8*1024), // 短链接, 不需要很大的缓冲区
	)
	if err != nil {
		panic(err.Error())
	}

	go func() {
		if err = forAccept.Run(); err != nil {
			panic(err.Error())
		}
	}()
	if err = forNewFd.Run(); err != nil {
		panic(err.Error())
	}
}
