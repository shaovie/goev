package main

import (
	"fmt"
	"sync"
	"syscall"

	"github.com/shaovie/goev"
)

var (
	buffPool *sync.Pool
)

const httpResp = "HTTP/1.1 200 OK\r\nConnection: close\r\nContent-Length: 5\r\n\r\nhello"

type Http struct {
	goev.Event
	m [4096]byte // test memory leak
}

func (h *Http) OnOpen(fd *goev.Fd, now int64) bool {
    // fd.SetNoDelay(1) // New socket has been set to non-blocking
    if err := h.GetReactor().AddEvHandler(h, fd.Fd(), goev.EV_IN); err != nil {
        return false
    }
	return true
}
func (h *Http) OnRead(fd *goev.Fd, now int64) bool {
    buf := buffPool.Get().([]byte) // just read
    defer buffPool.Put(buf)

	readN := 0
	for {
		if readN >= cap(buf) { // alloc new buff to read
			break
		}
		n, err := fd.Read(buf[readN:])
		if err != nil {
			if err == syscall.EAGAIN { // epoll ET mode
				break
			}
			fmt.Println("read: ", err.Error())
			return false
		}
		if n > 0 { // n > 0
			readN += n
		} else { // n == 0 connection closed,  will not < 0
			if readN == 0 {
				fmt.Println("peer closed. ", n)
			}
			return false
		}
	}
	fd.Write([]byte(httpResp)) // Connection: close
	return false               // will goto OnClose
}
func (h *Http) OnClose(fd *goev.Fd) {
	fd.Close()
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
	_, err = goev.NewAcceptor(forAccept, forNewFd, func() goev.EvHandler {return new(Http)},
		":2023",
		goev.ListenBacklog(256),
		goev.RecvBuffSize(8*1024), // 短链接, 不需要很大的缓冲区
	)
	if err != nil {
		panic(err.Error())
	}

	//= https
	_, err = goev.NewAcceptor(forAccept, forNewFd, func() goev.EvHandler {return new(Https)},
		":2024",
		goev.ListenBacklog(256),
		goev.RecvBuffSize(8*1024), // 短链接, 不需要很大的缓冲区
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
