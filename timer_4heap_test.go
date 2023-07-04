package goev

import (
	"fmt"
	"math/rand"
	"testing"
)

type fheapTimer struct {
	Event
}

func (t *fheapTimer) OnTimeout(now int64) bool {
	return false
}

func TestTimer4Heap_Algo(t *testing.T) {
	t4h := newTimer4Heap(1024)

	for i := 0; i < 200; i++ {
		delay := rand.Int63() % 200
		t4h.scheduleTest(&fheapTimer{}, delay, 0)
	}
	for i := 0; i < 200; i++ {
		ti, _ := t4h.popOne(0, 10000000)
		fmt.Println(ti.expiredAt)
	}
	fmt.Println("len", t4h.size())
}
