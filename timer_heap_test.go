package goev

import (
	"fmt"
	"math/rand"
	"os"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

var counter atomic.Int32
var showlog atomic.Int32

type timingOuput struct {
	Event

	idx       int
	expiredAt int64
	interval  int64
}

func newTimingOuput(idx int, expiredAt, interval int64) EvHandler {
	return &timingOuput{
		idx:       idx,
		expiredAt: expiredAt,
		interval:  interval,
	}
}
func (t *timingOuput) OnTimeout(now int64) bool {
	diff := now - t.expiredAt
	if rand.Int63()%100 < 10 {
		// fmt.Printf("%d exit, diff=%d interval=%d\n", t.idx, diff, t.interval)
		counter.Add(-1)
		return false
	}
	if diff > 5 {
		fmt.Println("diff =", diff)
	}
	if showlog.Load() == 1 {
		fmt.Printf("timer %d alive interval=%d\n", t.idx, t.interval)
	}

	t.expiredAt = now + t.interval
	return true
}

type exitTimer struct {
	Event
	t *testing.T
}

func (t *exitTimer) OnTimeout(now int64) bool {
	c := counter.Load()
	fmt.Println("-------------------exit counter=", c)
	if c < 1 {
		os.Exit(0)
		return false
	}
	if c < 10 {
		showlog.Store(1)
	}
	return true
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

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		r.Run()
		wg.Done()
	}()

	var idx int
	for i := 0; i < 2000; i++ {
		now := time.Now().UnixMilli()
		sec := rand.Int63()%5 + 1
		msec := rand.Int63()%1000 + 10
		expiredAt := now + sec*1000 + msec
		err := r.SchedueTimer(newTimingOuput(idx, expiredAt, msec), sec*1000+msec, msec)
		if err != nil {
			fmt.Printf("schedule err %s\n", err.Error())
		}
		counter.Add(1)
		idx += 1
	}
	go func() {
		for i := 0; i < 100; i++ {
			for j := 0; j < 100; j++ {
				now := time.Now().UnixMilli()
				sec := rand.Int63()%5 + 1
				msec := rand.Int63()%1000 + 10
				expiredAt := now + sec*1000 + msec
				err := r.SchedueTimer(newTimingOuput(idx, expiredAt, msec), sec*1000+msec, msec)
				if err != nil {
					fmt.Printf("schedule err %s\n", err.Error())
				}
				counter.Add(1)
				idx += 1
			}
			time.Sleep(20 * time.Millisecond)
		}
		r.SchedueTimer(&exitTimer{t: t}, 5000, 1000)
		fmt.Println("======================schedule exit timer")
	}()
	wg.Wait()
}

type heapTimer struct {
	Event
}

func (t *heapTimer) OnTimeout(now int64) bool {
	return false
}

func TestTimerHeap_Algo(t *testing.T) {
	t4h := newTimerHeap(1024)

	for i := 0; i < 200; i++ {
		delay := rand.Int63() % 200
		t4h.scheduleTest(&heapTimer{}, delay, 0)
	}
	for i := 0; i < 200; i++ {
		ti, _ := t4h.popOne(0, 10000000)
		fmt.Println(ti.expiredAt)
	}
	fmt.Println("len", t4h.size())
}
