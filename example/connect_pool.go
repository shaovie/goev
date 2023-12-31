package main

import (
	"fmt"
	"math/rand"
	"sync"
	"sync/atomic"
	"time"

	"github.com/shaovie/goev"
)

var (
	buffPool        *sync.Pool
	pushCounter     atomic.Int32
	forNewFdReactor *goev.Reactor
)

type AsyncPushLog struct {
	goev.ConnectPoolItem
}

func (s *AsyncPushLog) OnOpen() bool {
	if err := forNewFdReactor.AddEvHandler(s, s.Fd(), goev.EvIn); err != nil {
		fmt.Printf("error: fd %d %s\n", s.Fd(), err.Error())
		return false
	}
	return true
}
func (s *AsyncPushLog) OnRead() bool {
	data, n, _ := s.Read()
	if n == 0 { // Abnormal connection
		return false
	}
	_ = data
	return true
}
func (s *AsyncPushLog) Push(log string) {
	s.Write([]byte(log))
}
func (s *AsyncPushLog) OnClose() {
	fmt.Printf("closed\n")
	s.Closed()
	s.Destroy(s)
}
func doPush(pusher *AsyncPushLog) {
	msec := rand.Int63()%50 + 1
	time.Sleep(time.Millisecond * time.Duration(msec))
	pusher.Push("hello 6379")
	pushCounter.Add(1)
	pusher.GetPool().Release(pusher)
}
func main() {
	fmt.Printf("hello boy\n")
	buffPool = &sync.Pool{
		New: func() any {
			return make([]byte, 4096)
		},
	}
	// 1. reactor
	r, err := goev.NewReactor(
		goev.EvFdMaxSize(0), // default val
		goev.EvPollNum(5),
		goev.TimerHeapInitSize(100),
	)
	forNewFdReactor = r
	if err != nil {
		panic(err.Error())
	}

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		r.Run()
		wg.Done()
	}()
	// 2. connector
	c, err := goev.NewConnector(forNewFdReactor, goev.SockRcvBufSize(8*1024))
	if err != nil {
		panic(err.Error())
	}

	// 3. connect_pool
	cp, err := goev.NewConnectPool(
		c, "127.0.0.1:6379", 40, 10, 100,
		1000, 200,
		func() goev.ConnectPoolHandler { return new(AsyncPushLog) },
	)
	if err != nil {
		panic(err.Error())
	}

	time.Sleep(time.Millisecond * 500)
	fmt.Printf("conn pool idle: %d after 500msec\n", cp.IdleNum())

	//gp := NewGoPool(100, 100, 100)
	for i := 0; i < 10000; {
		conn := cp.Acquire()
		if conn == nil {
			time.Sleep(time.Millisecond * 1)
			continue
		}
		go doPush(conn.(*AsyncPushLog))
		i++
	}

	for {
		time.Sleep(time.Millisecond * 500)
		c := pushCounter.Load()
		fmt.Printf("conn pool idle: %d end. live: %d, push: %d\n", cp.IdleNum(), cp.LiveNum(), c)
		if c >= 1000 {
			break
		}
	}
	wg.Wait()
}
