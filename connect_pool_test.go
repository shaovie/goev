package goev

import (
	"fmt"
	"math/rand"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/shaovie/goev/netfd"
)

var (
	buffPool    *sync.Pool
	pushCounter atomic.Int32
)

type AsyncPushLog struct {
	ConnectPoolItem

	fd int
}

func (s *AsyncPushLog) OnOpen(fd int, now int64) bool {
	if err := s.GetReactor().AddEvHandler(s, fd, EvIn); err != nil {
		fmt.Printf("error: fd %d %s\n", fd, err.Error())
		return false
	}
	s.fd = fd
	return true
}
func (s *AsyncPushLog) OnRead(fd int, nio IOReadWriter, now int64) bool {
	_, err := nio.Read(fd)
	if nio.Closed() || err == ErrRcvBufOutOfLimit { // Abnormal connection
		return false
	}
	return true
}
func (s *AsyncPushLog) Push(log string) {
	netfd.Write(s.fd, []byte(log))
}
func (s *AsyncPushLog) OnClose(fd int) {
	fmt.Printf("closed\n")
	netfd.Close(fd)
	s.Closed()
}
func doPush(pusher *AsyncPushLog) {
	msec := rand.Int63()%50 + 1
	time.Sleep(time.Millisecond * time.Duration(msec))
	pusher.Push("hello 6379")
	pushCounter.Add(1)
	pusher.GetPool().Release(pusher)
}
func TestConnectPool(t *testing.T) {
	fmt.Printf("hello boy\n")
	buffPool = &sync.Pool{
		New: func() any {
			return make([]byte, 4096)
		},
	}
	// 1. reactor
	r, err := NewReactor(
		EvDataArrSize(0), // default val
		EvPollNum(5),
		EvReadyNum(8), // just timer
		TimerHeapInitSize(100),
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
	// 2. connector
	c, err := NewConnector(r, SockRcvBufSize(8*1024))
	if err != nil {
		panic(err.Error())
	}

	// 3. connect_pool
	cp, err := NewConnectPool(
		c, "127.0.0.1:6379", 40, 10, 100,
		1000, 200,
		func() ConnectPoolHandler { return new(AsyncPushLog) },
	)
	if err != nil {
		panic(err.Error())
	}

	time.Sleep(time.Millisecond * 500)
	fmt.Printf("conn pool idel: %d after 500msec\n", cp.IdleNum())

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
