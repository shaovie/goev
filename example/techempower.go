package main

import (
	"flag"
	"fmt"
	"os"
	"runtime"
	"sync/atomic"
	"time"

	"github.com/shaovie/goev"
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

func (h *Http) OnOpen() bool {
	// AddEvHandler 尽量放在最后, (OnOpen 和ORead可能不在一个线程)
	if err := forNewFdReactor.AddEvHandler(h, h.Fd(), goev.EvIn); err != nil {
		return false
	}
	return true
}
func (h *Http) OnRead() bool {
	_, n, _ := h.Read()
	if n == 0 { // Abnormal connection
		return false
	}

	buf := h.WriteBuff()[:0]
	buf = append(buf, httpRespHeader...)
	buf = append(buf, []byte(liveDate.Load().(string))...)
	buf = append(buf, httpRespContentLength...)
	h.Write(buf)
	return true
}
func (h *Http) OnClose() {
	h.Destroy(h)
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
	if runtime.NumCPU() > 33 {
		evPollNum = runtime.NumCPU()/2 + 4
	} else if runtime.NumCPU() > 15 {
		evPollNum = runtime.NumCPU()/2 + 2
	} else if runtime.NumCPU() > 3 {
		evPollNum = runtime.NumCPU() / 2
	}
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
