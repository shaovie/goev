package main

import (
	"fmt"
	"io"
	"os"
	"runtime"
	"strconv"
	"sync"
	"unsafe"

	"github.com/shaovie/goev"
	"github.com/shaovie/goev/netfd"
)

var (
	reactor     *goev.Reactor
	asynBufPool sync.Pool
)

const httpHeaderS = "HTTP/1.1 200 OK\r\nConnection: keep-alive\r\nServer: goev\r\n" +
	"Content-Type: application/octet-stream\r\nContent-Length: "

type Conn struct {
	goev.IOHandle

	f *os.File
}

func (c *Conn) OnOpen(fd int) bool {
	netfd.SetSendBuffSize(fd, 1*4096)
	// AddEvHandler 尽量放在最后, (OnOpen 和ORead可能不在一个线程)
	if err := reactor.AddEvHandler(c, fd, goev.EvIn); err != nil {
		return false
	}
	return true
}
func (c *Conn) OnRead() bool {
	_, n, _ := c.Read()
	if n == 0 { // Abnormal connection
		fmt.Println("peer closed")
		return false
	}

	f, err := os.Stat("./downloadfile")
	if err != nil {
		return false
	}
	buf := c.WriteBuff()[:0]
	buf = append(buf, (unsafe.Slice(unsafe.StringData(httpHeaderS), len(httpHeaderS)))...)
	buf = strconv.AppendInt(buf, f.Size(), 10)
	buf = append(buf, []byte("\r\n\r\n")...)
	writen, _ := c.Write(buf)
	if writen < len(buf) {
		bf := asynBufPool.Get().([]byte)
		n = copy(bf, buf[writen:])
		c.AsyncWrite(c, goev.AsyncWriteBuf{
			Len: n,
			Buf: bf,
		})
	}
	c.f, _ = os.Open("./downloadfile")
	c.ScheduleTimer(c, 0, 200)
	fmt.Println("start")
	return true
}
func (c *Conn) OnTimeout(now int64) bool {
	if c.Fd() < 0 {
		fmt.Println("fd closed")
		return false
	}
	//fmt.Println("timeout", c.AsyncLastPartialWriteTime(), c.AsyncWaitWriteQLen())
	if c.AsyncWaitWriteQLen() > 0 {
		return true
	}
	bf := asynBufPool.Get().([]byte)
	n, err := c.f.Read(bf)
	if n > 0 {
		c.AsyncWrite(c, goev.AsyncWriteBuf{
			Len: n,
			Buf: bf,
		})
	} else if err == io.EOF {
		c.AsyncWrite(c, goev.AsyncWriteBuf{
			Flag: 1, // end flag
			Len:  0,
			Buf:  nil,
		})
		fmt.Println("EOF")
		return false
	}
	if c.AsyncWaitWriteQLen() > 0 {
		fmt.Println("AsyncWaitWriteQLen: ", c.AsyncWaitWriteQLen())
	}
	return true
}
func (c *Conn) OnWrite() bool {
	c.AsyncOrderedFlush(c)
	fmt.Println("on write")
	return true
}
func (c *Conn) OnAsyncWriteBufDone(bf []byte, flag int) {
	if flag == 0 {
		asynBufPool.Put(bf)
	} else if flag == 1 && c.Fd() > 0 { // send completely
		fmt.Println("send completely")
		reactor.RemoveEvHandler(c, c.Fd()) // remove at first, then call OnClose
		c.OnClose()                        // (NOTE: avoid IOHandle.Destroy(eh.OnAsyncWriteBufDone) endless loop)
		return
	}
}
func (c *Conn) OnClose() {
	fmt.Println("on close")
	if c.Fd() != -1 {
		if c.f != nil {
			c.f.Close()
		}
		c.CancelTimer(c)
		netfd.Close(c.Fd())
		c.Destroy(c)
	}
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
		goev.EvPollNum(runtime.NumCPU()*2-1),
	)
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

	if err = reactor.Run(); err != nil {
		panic(err.Error())
	}
}
