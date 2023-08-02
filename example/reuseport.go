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
	reactor               *goev.Reactor
)

const httpHeaderS = "HTTP/1.1 200 OK\r\nConnection: keep-alive\r\nServer: goev\r\nContent-Type: text/plain\r\nDate: "
const contentLengthS = "\r\nContent-Length: 13\r\n\r\nHello, World!"

type Http struct {
	goev.Event
}

func (h *Http) OnOpen(fd int) bool {
	// AddEvHandler 尽量放在最后, (OnOpen 和ORead可能不在一个线程)
	if err := reactor.AddEvHandler(h, fd, goev.EvIn); err != nil {
		return false
	}
	return true
}
func (h *Http) OnRead(fd int, nio goev.IOReadWriter) bool {
    // 23.08.02 这里使用 goev.IOReadWriter 进行IO操作,非常影响性能(整体吞吐量下降20%, 应该是被调度了), 还没查清原因
	_, err := nio.Read(fd)
	if nio.Closed() || err == goev.ErrRcvBufOutOfLimit { // Abnormal connection
		return false
	}

	nio.InitWrite()
    nio.Append(httpRespHeader)
    nio.Append([]byte(liveDate.Load().(string)))
	nio.Append(httpRespContentLength)
	nio.Write(fd)
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

	evPollNum := runtime.NumCPU()*2 - 1
	var err error
	reactor, err = goev.NewReactor(
		goev.EvDataArrSize(20480), // default val
		goev.EvPollNum(evPollNum),
	)
	if err != nil {
		panic(err.Error())
	}
	// ReusePort 模式下, 创建N个相同的acceptor(listener fd), 注册不到同的evPool中,
	// 内核会将新链接调度到不同的listener fd的全链接队列中去), 这样不会出现惊群效应
	for i := 0; i < evPollNum; i++ {
		_, err = goev.NewAcceptor(reactor,
			func() goev.EvHandler { return new(Http) },
			":8080",
			goev.ListenBacklog(128),
			goev.ReusePort(true),
			//goev.SockRcvBufSize(16*1024), // 短链接, 不需要很大的缓冲区
		)
		if err != nil {
			panic(err.Error())
		}
	}

	go updateLiveSecond()
	if err = reactor.Run(); err != nil {
		panic(err.Error())
	}
}
