package main

import (
    "github.com/shaovie/goev"
)

type Conn struct {
    goev.NullEvHandler
}

func (c *Conn) OnOpen(fd *goev.Fd) bool {
    //
}
func (c *Conn) OnRead(fd *goev.Fd) bool {
}
func (c *Conn) OnClose(fd *goev.Fd) bool {
}


func main() {
    r, err := goev.NewReactor(
        goev.EvPollSize(1024),
        goev.EvPollThreadNum(2),
    )
    if err != nil {
        panic(err.Error())
    }
    if err = r.Open(); err != nil {
        panic(err.Error())
    }
    acceptor, err := goev.NewAcceptor(
        goev.ListenBacklog(256),
        goev.RecvBuffSize(8*1024), // 短链接, 不需要很大的缓冲区
    )
    if err != nil {
        panic(err.Error())
    }
    acceptor.Open(r, func() goev.EvHandler {
            return new(Conn)
        },
        ":2023",
        goev.EV_IN,
    )

    if err = r.Run(); err != nil {
        panic(err.Error())
    }
}
