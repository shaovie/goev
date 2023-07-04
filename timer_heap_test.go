package goev

import (
    "os"
    "fmt"
    "time"
	"testing"
	"math/rand"
)

type timingOuput struct {
    Event

    idx int
    expiredAt int64
    interval int64
}

func newTimingOuput(idx int, expiredAt, interval int64) EvHandler {
    return &timingOuput{
        idx: idx,
        expiredAt: expiredAt,
        interval: interval,
    }
}
func (t *timingOuput) OnTimeout(now int64) bool {
    diff := now - t.expiredAt
    if rand.Int63() % 100 < 10 {
        // fmt.Printf("%d exit, diff=%d interval=%d\n", t.idx, diff, t.interval)
        return false
    }
    if diff > 5 {
        fmt.Println("diff =", diff)
    }
    
    t.expiredAt = now + t.interval
    return true
}
type exitTimer struct {
    Event
    t *testing.T
}
func (t *exitTimer) OnTimeout(now int64) bool {
    fmt.Println("-------------------exit")
    os.Exit(0)
    return false
}

func TestTimerHeap(t *testing.T) {
    fmt.Println("hello boy")
	r, err := NewReactor(
		EvDataArrSize(0), // default val
		EvPollNum(10),
		EvReadyNum(8), // just timer
		TimerHeapInitSize(10000),
	)
	if err != nil {
		panic(err.Error())
	}

    idx := 0
    for i := 0; i < 10000; i++ {
        now := time.Now().UnixMilli()
        sec := rand.Int63() % 5
        msec := rand.Int63() % 1000
        expiredAt := now + sec * 1000 + msec
        err := r.SchedueTimer(newTimingOuput(idx, expiredAt, msec), sec * 1000 + msec, msec)
        if err != nil {
            t.Fatalf("schedule err %s", err.Error())
        }
        idx += 1
    }
    go func() {
        for i := 0; i < 100; i++ {
            for j := 0; j < 100; j++ {
                now := time.Now().UnixMilli()
                sec := rand.Int63() % 5
                msec := rand.Int63() % 1000
                expiredAt := now + sec * 1000 + msec
                err := r.SchedueTimer(newTimingOuput(idx, expiredAt, msec), sec * 1000 + msec, msec)
                if err != nil {
                    t.Fatalf("schedule err %s", err.Error())
                }
                idx += 1
            }
            time.Sleep(20*time.Millisecond)
        }
        r.SchedueTimer(&exitTimer{t: t}, 6000, 0)
        fmt.Println("======================schedule exit timer")
    }()
    r.Run()
}
