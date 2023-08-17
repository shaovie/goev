package goev

import (
	"fmt"
	"golang.org/x/sys/unix"
	"math/rand"
	"testing"
	"time"
)

type fheapTimerOnce struct {
	IOHandle
}

func (t *fheapTimerOnce) OnOpen() bool {
	t.ScheduleTimer(t, 1000, 0)
	return true
}
func (t *fheapTimerOnce) OnTimeout(now int64) bool {
	fmt.Println("once now", time.Now().String(), time.UnixMilli(now).String())
	return true
}

type fheapTimer struct {
	IOHandle
}

func (t *fheapTimer) OnOpen() bool {
	t.ScheduleTimer(t, 1000, 2020)
	return true
}
func (t *fheapTimer) OnTimeout(now int64) bool {
	fmt.Println("now", time.Now().String(), time.UnixMilli(now).String())
	return true
}

func TestTimer4Heap_Algo(t *testing.T) {
	t4h := newTimer4Heap(1024)

	var obj EvHandler
	for i := 0; i < 200; i++ {
		delay := rand.Int63()%200 + 2
		obj = &fheapTimer{}
		t4h.scheduleTest(obj, delay, 0)
	}
	t4h.cancel(obj)
	for i := 0; i < 200; i++ {
		ti, _ := t4h.popOne(0, 10000000)
		fmt.Println(ti.expiredAt)
	}
	fmt.Println("len", t4h.size())
}
func TestTimer4Heap(t *testing.T) {
	reactor, _ := NewReactor(
		EvFdMaxSize(20480), // default val
		EvPollNum(2),
	)

	fd, _ := unix.Eventfd(0, unix.EFD_NONBLOCK|unix.EFD_CLOEXEC)
	ft := &fheapTimer{}
	reactor.AddEvHandler(ft, fd, EvIn)
	ft.OnOpen()

	fd, _ = unix.Eventfd(0, unix.EFD_NONBLOCK|unix.EFD_CLOEXEC)
	ft2 := &fheapTimerOnce{}
	reactor.AddEvHandler(ft2, fd, EvIn)
	ft2.OnOpen()

	go func() {
		reactor.Run()
	}()
	time.Sleep(time.Second * 10)
}
