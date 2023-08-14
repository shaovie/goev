package main

import (
	"flag"
	"fmt"
	"os"
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

// Launch args
var (
	evPollNum int = runtime.NumCPU()
	procNum   int = runtime.NumCPU() * 2
)

func usage() {
	fmt.Println(`
    Server options:
    -c N                   Evpoll num
    -p N                   PROC num

    Common options:
    -h                     Show this message
    `)
	os.Exit(0)
}
func parseFlag() {
	flag.IntVar(&evPollNum, "c", evPollNum, "evpoll num.")
	flag.IntVar(&procNum, "p", procNum, "proc num.")

	flag.Usage = usage
	flag.Parse()
}

const httpHeaderS = "HTTP/1.1 200 OK\r\nConnection: keep-alive\r\nServer: goev\r\nContent-Type: text/plain\r\nDate: "
const contentLengthS = "\r\nContent-Length: 13\r\n\r\nHello, World!"

type Http struct {
	goev.IOHandle
}

func (h *Http) OnOpen(fd int) bool {
	// AddEvHandler 尽量放在最后, (OnOpen 和ORead可能不在一个线程)
	if err := forNewFdReactor.AddEvHandler(h, fd, goev.EvIn); err != nil {
		return false
	}
	return true
}
func (h *Http) OnRead() bool {
	_, n, _ := h.Read()
	if n == 0 { // Abnormal connection
		return false
	}
	go h.AsyncHandle()
	return true
}
func (h *Http) OnWrite() bool {
	h.AsyncOrderedFlush(h)
	return true
}
func (h *Http) AsyncHandle() {
	buf := make([]byte, 0, 256)
	buf = append(buf, httpRespHeader...)
	buf = append(buf, []byte(liveDate.Load().(string))...)
	buf = append(buf, httpRespContentLength...)
	h.AsyncWrite(h, buf)
}
func (h *Http) OnClose() {
	if h.Fd() != -1 {
		netfd.Close(h.Fd())
		h.Destroy(h)
	}
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
	parseFlag()
	fmt.Printf("hello boy! GOMAXPROCS=%d evpoll num=%d\n", procNum, evPollNum)
	runtime.GOMAXPROCS(procNum)

	liveDate.Store(time.Now().Format("Mon, 02 Jan 2006 15:04:05 GMT"))
	ticker = time.NewTicker(time.Millisecond * 1000)

	httpRespHeader = []byte(httpHeaderS)
	httpRespContentLength = []byte(contentLengthS)

	forAcceptReactor, err := goev.NewReactor(
		goev.EvFdMaxSize(20480), // default val
		goev.EvPollNum(1),
	)
	if err != nil {
		panic(err.Error())
	}
	forNewFdReactor, err = goev.NewReactor(
		goev.EvFdMaxSize(20480), // default val
		goev.EvPollNum(evPollNum),
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
