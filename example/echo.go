package main

import (
	"fmt"
	"runtime"

	"github.com/shaovie/goev"
	"github.com/shaovie/goev/netfd"
)

var (
	reactor     *goev.Reactor
)

type Conn struct {
	goev.IOHandle
}

func (c *Conn) OnOpen(fd int) bool {
	netfd.SetNoDelay(fd, 1)
	// AddEvHandler 尽量放在最后, (OnOpen 和ORead可能不在一个线程)
	if err := reactor.AddEvHandler(c, fd, goev.EvIn); err != nil {
		return false
	}
	return true
}
func (c *Conn) OnRead() bool {
	buf, n, _ := c.Read()
    if n > 0 {
        c.Write(buf[0:n])
    } else if n == 0 { // Abnormal connection
		return false
	}
	return true
}
func (c *Conn) OnClose() {
	if c.Fd() != -1 {
		netfd.Close(c.Fd())
		c.Destroy(c)
	}
}

func main() {
	fmt.Println("hello boy")
	runtime.GOMAXPROCS(runtime.NumCPU()*2) // 留一部分给网卡中断

	var err error
	reactor, err = goev.NewReactor(
		goev.EvDataArrSize(4096), // default val
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
