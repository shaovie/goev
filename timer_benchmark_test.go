package goev

import (
	"math/rand"
	"testing"
	"time"
)

type nullTimer struct {
	Event
}

func (t *nullTimer) OnTimeout(now int64) bool {
	return false
}
func BenchmarkTimer_Heap(b *testing.B) {
	cases := []struct {
		name string
		N    int // the data size (i.e. number of existing timers)
	}{
		{"N-1k", 1000},
		{"N-5k", 5000},
		{"N-10k", 10000},
		{"N-50k", 50000},
	}
	for _, c := range cases {
		b.Run(c.name, func(b *testing.B) {
			th := newTimerHeap(1024)
			for i := 0; i < c.N; i++ {
				delay := rand.Int63() % 5 * 1000 // sec
				interval := rand.Int63() % 3000
				th.schedule(&nullTimer{}, delay, interval)
			}
			now := time.Now().UnixMilli()
			for i := 0; i < c.N; i++ {
				th.handleExpired(now + int64(i))
			}
		})
	}
}
func BenchmarkTimer_FHeap(b *testing.B) {
	cases := []struct {
		name string
		N    int // the data size (i.e. number of existing timers)
	}{
		{"N-1k", 1000},
		{"N-5k", 5000},
		{"N-10k", 10000},
		{"N-50k", 50000},
	}
	for _, c := range cases {
		b.Run(c.name, func(b *testing.B) {
			th := newTimer4Heap(1024)
			for i := 0; i < c.N; i++ {
				delay := rand.Int63() % 5 * 1000 // sec
				interval := rand.Int63() % 3000
				th.schedule(&nullTimer{}, delay, interval)
			}
			now := time.Now().UnixMilli()
			for i := 0; i < c.N; i++ {
				th.handleExpired(now + int64(i))
			}
		})
	}
}
