package goev

import (
	"fmt"
	"runtime"
	"strconv"
	"sync"
	"testing"
)

type Scanner struct {
	IOHandle

	port int
}

func (s *Scanner) OnOpen(fd int) bool {
	fmt.Printf("port %d open\n", s.port)
	return false
}
func (s *Scanner) OnConnectFail(err error) {
	//fmt.Printf("port %d close %s\n", s.port, err.Error())
}
func (s *Scanner) OnClose() {
	s.Destroy(s)
}
func TestConnector(t *testing.T) {
	fmt.Println("hello boy")
	r, err := NewReactor(
		EvPollNum(runtime.NumCPU()*2),
		TimerHeapInitSize(10000),
	)
	if err != nil {
		panic(err.Error())
	}

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		r.Run()
		wg.Done()
	}()
	c, err := NewConnector(r, SockRcvBufSize(8*1024))
	for i := 80; i < 65535; i++ {
		c.Connect("108.138.105.100:"+strconv.FormatInt(int64(i), 10), &Scanner{port: i}, 3000)
	}

	wg.Wait()
}
