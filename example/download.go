package main

import (
	"fmt"
	"io"
	"os"
	"runtime"
	"strconv"
	"sync"
	"time"
	"unsafe"

	"github.com/shaovie/goev"
	"github.com/shaovie/goev/netfd"
)

var (
	reactor     *goev.Reactor
	asynBufPool sync.Pool
)

const (
	ConfigSpeed = 10
)

const httpHeaderS = "HTTP/1.1 200 OK\r\nConnection: keep-alive\r\nServer: goev\r\n" +
	"Content-Type: application/octet-stream\r\nContent-Length: "

type Conn struct {
	goev.IOHandle

	f         *os.File
	confSpeed int64
}

func (c *Conn) OnOpen() bool {
	netfd.SetSendBuffSize(c.Fd(), 4*4096)
	// AddEvHandler 尽量放在最后, (OnOpen 和ORead可能不在一个线程)
	if err := reactor.AddEvHandler(c, c.Fd(), goev.EvIn); err != nil {
		return false
	}

	// Must after eactor.AddEvHandler
	confSpeed, ok := c.PCachedGet(ConfigSpeed)
	if ok {
		c.confSpeed = confSpeed.(int64)
		fmt.Println("conf speed", c.confSpeed)
	}
	return true
}
func (c *Conn) OnRead() bool {
	_, n, _ := c.Read()
	if n == 0 { // Abnormal connection
		fmt.Println("peer closed")
		return false
	}

	f, err := os.Stat("./download")
	if err != nil {
		return false
	}
	buf := c.WriteBuff()[:0]
	buf = append(buf, (unsafe.Slice(unsafe.StringData(httpHeaderS), len(httpHeaderS)))...)
	buf = strconv.AppendInt(buf, f.Size(), 10)
	buf = append(buf, []byte("\r\n\r\n")...)
	c.Write(buf)
	c.f, _ = os.Open("./download")
	c.ScheduleTimer(c, 0, 200)
	fmt.Println("start")
	return true
}
func (c *Conn) OnTimeout(now int64) bool {
	if c.Fd() < 0 {
		fmt.Println("fd closed")
		return false
	}
	confSpeed, ok := c.PCachedGet(ConfigSpeed)
	if ok && confSpeed.(int64) != c.confSpeed {
		c.confSpeed = confSpeed.(int64)
		fmt.Println("conf speed update", c.confSpeed)
	}
	if c.AsyncWaitWriteQLen() > 0 { // wait
		return true
	}
	bf := c.WriteBuff()
	n, err := c.f.Read(bf[0:4096])
	if n > 0 {
		c.Write(bf[0:n])
	} else if err == io.EOF {
		c.f.Close()
		c.f = nil
		fmt.Println("EOF")
		return false
	}
	return true
}
func (c *Conn) OnWrite() bool {
	c.AsyncOrderedFlush(c)
	fmt.Println("on write")
	return true
}
func (c *Conn) OnClose() {
	fmt.Println("on close")
	if c.f != nil {
		c.f.Close()
		c.f = nil
	}
	c.CancelTimer(c)
	c.Destroy(c)
}

func main() {
	fmt.Println("hello boy")
	runtime.GOMAXPROCS(runtime.NumCPU()*2 - 1) // 留一部分给网卡中断

	asynBufPool.New = func() any {
		return make([]byte, 1024)
	}

	var err error
	reactor, err = goev.NewReactor(
		goev.EvFdMaxSize(2048), // default val
		goev.EvPollNum(runtime.NumCPU()*2),
	)
	reactor.InitPollSyncOpt(goev.PollSyncCache, goev.PollSyncCacheOpt{
		ID:    ConfigSpeed,
		Value: int64(1000),
	})
	if err != nil {
		panic(err.Error())
	}
	//= http
	_, err = goev.NewAcceptor(reactor, func() goev.EvHandler { return new(Conn) },
		":8080",
		goev.ListenBacklog(128),
	)
	if err != nil {
		panic(err.Error())
	}
	go func() {
		time.Sleep(time.Second * 10)
		fmt.Println("to update conf speed")
		reactor.PollSyncOpt(goev.PollSyncCache, goev.PollSyncCacheOpt{
			ID:    ConfigSpeed,
			Value: int64(5000),
		})
	}()

	if err = reactor.Run(); err != nil {
		panic(err.Error())
	}
}
