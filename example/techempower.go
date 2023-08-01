package main

import (
	"fmt"
	"runtime"
	"sync/atomic"
	"time"

	"github.com/shaovie/goev"
	"github.com/shaovie/goev/netfd"
)

var (
	httpRespHeader        []byte
	httpRespContentLength []byte
	ticker                *time.Ticker
	liveDate              atomic.Value
	forNewFdReactor       *goev.Reactor
)

const httpHeaderS = "HTTP/1.1 200 OK\r\nConnection: keep-alive\r\nServer: goev\r\nContent-Type: text/plain\r\nDate: "
const contentLengthS = "\r\nContent-Length: 13\r\n\r\nHello, World!"

type Http struct {
	goev.Event
}

func (h *Http) OnOpen(fd int) bool {
	// AddEvHandler 尽量放在最后, (OnOpen 和ORead可能不在一个线程)
	if err := forNewFdReactor.AddEvHandler(h, fd, goev.EvIn); err != nil {
		return false
	}
	return true
}
func (h *Http) OnRead(fd int, nio goev.IOReadWriter) bool {
	_, err := nio.Read(fd)
	if nio.Closed() || err == goev.ErrRcvBufOutOfLimit { // Abnormal connection
		return false
	}

	nio.InitWrite().Append(httpRespHeader).
		Append([]byte(liveDate.Load().(string))).
		Append(httpRespContentLength).
		Write(fd)
	return true
}
func (h *Http) OnClose(fd int) {
	netfd.Close(fd)
}

func updateLiveSecond() {
	for {
		select {
		case now := <-ticker.C:
			liveDate.Store(now.Format("Mon, 02 Jan 2006 15:04:05 GMT"))
		}
	}
}

func main() {
	fmt.Println("hello boy")
	runtime.GOMAXPROCS(runtime.NumCPU()*2 - 1) // 留一部分给网卡中断

	liveDate.Store(time.Now().Format("Mon, 02 Jan 2006 15:04:05 GMT"))
	ticker = time.NewTicker(time.Millisecond * 1000)

	httpRespHeader = []byte(httpHeaderS)
	httpRespContentLength = []byte(contentLengthS)

	forAcceptReactor, err := goev.NewReactor(
		goev.EvDataArrSize(20480), // default val
		goev.EvPollNum(1),
		goev.EvReadyNum(8), // only accept fd
	)
	if err != nil {
		panic(err.Error())
	}
	forNewFdReactor, err = goev.NewReactor(
		goev.EvDataArrSize(20480), // default val
		goev.EvPollNum(runtime.NumCPU()*2-1),
		goev.EvReadyNum(512), // auto calc
	)
	if err != nil {
		panic(err.Error())
	}
	//= http
	_, err = goev.NewAcceptor(forAcceptReactor, func() goev.EvHandler { return new(Http) },
		":8080",
		goev.ListenBacklog(512),
		//goev.SockRcvBufSize(16*1024), // 短链接, 不需要很大的缓冲区
	)
	if err != nil {
		panic(err.Error())
	}

	go updateLiveSecond()
	go func() {
		if err = forAcceptReactor.Run(); err != nil {
			panic(err.Error())
		}
	}()
	if err = forNewFdReactor.Run(); err != nil {
		panic(err.Error())
	}
}
