package goev

import (
	"math/rand"
	"testing"
	"time"
)

`
BenchmarkTimer_Heap/N-1k-2         	1000000000	         0.0005251 ns/op
BenchmarkTimer_Heap/N-5k-2         	1000000000	         0.002146 ns/op
BenchmarkTimer_Heap/N-10k-2        	1000000000	         0.004548 ns/op
BenchmarkTimer_Heap/N-50k-2        	1000000000	         0.03309 ns/op
BenchmarkTimer_FHeap/N-1k-2        	1000000000	         0.0002952 ns/op
BenchmarkTimer_FHeap/N-5k-2        	1000000000	         0.001791 ns/op
BenchmarkTimer_FHeap/N-10k-2       	1000000000	         0.003504 ns/op
BenchmarkTimer_FHeap/N-50k-2       	1000000000	         0.02721 ns/op
`

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
