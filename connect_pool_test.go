package goev

import (
	"sync"
    "time"
    "syscall"
	"testing"
	"sync/atomic"

    "goev/netfd"
)

var (
	buffPool *sync.Pool
    pushCounter atomic.Int32
)

type AsyncPushLog struct {
	Event

    fd int
}

func (s *AsyncPushLog) OnOpen(fd int, now int64) bool {
    if err := s.GetReactor().AddEvHandler(s, fd, EV_IN); err != nil {
        Error("fd %d %s", fd, err.Error())
        return false
    }
    s.fd = fd
    return true
}
func (s *AsyncPushLog) OnRead(fd int, now int64) bool {
    buf := buffPool.Get().([]byte) // just read
    defer buffPool.Put(buf)

	readN := 0
	for {
		if readN >= cap(buf) { // alloc new buff to read
			break
		}
		n, err := netfd.Read(fd, buf[readN:])
		if err != nil {
			if err == syscall.EAGAIN { // epoll ET mode
				break
			}
			Debug("read: %s", err.Error())
			return false
		}
		if n > 0 { // n > 0
			readN += n
		} else { // n == 0 connection closed,  will not < 0
			if readN == 0 {
				Debug("peer closed. %d", n)
			}
			return false
		}
	}
	return true
}
func (s *AsyncPushLog) Push(log string) {
    netfd.Write(s.fd, []byte(log))
}
func (s *AsyncPushLog) OnClose(fd int) {
    Debug("closed")
	netfd.Close(fd)
}
func doPush(pusher *AsyncPushLog, cp *ConnectPool) {
    pusher.Push("hello 6379")
    pushCounter.Add(1)
    cp.Release(pusher)
}
func TestConnectPool(t *testing.T) {
	Debug("hello boy")
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
	c, err := NewConnector(r, RecvBuffSize(8*1024))
    if err != nil {
		panic(err.Error())
    }

    // 3. connect_pool
    cp, err := NewConnectPool(c, "127.0.0.1:6379", 10, 10, 50, func() ConnHandler { return new(AsyncPushLog) })
    if err != nil {
		panic(err.Error())
    }

    time.Sleep(time.Millisecond * 500)
    Debug("conn pool size %d after 500msec", cp.Size())

    for i := 0; i < 1000; {
        conn := cp.Acquire()
        if conn == nil {
            time.Sleep(time.Millisecond * 1)
            continue
        }
        go doPush(conn.(*AsyncPushLog), cp)
        i += 1
    }

    for {
        time.Sleep(time.Millisecond * 500)
        c := pushCounter.Load()
        Debug("conn pool size %d end. counter=%d", cp.Size(), c)
        if c >= 1000 {
            break
        }
    }
	wg.Wait()
}
