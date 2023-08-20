package goev

import (
	"fmt"
	"math/rand"
	"sync"
	"testing"
)

func TestMempool(t *testing.T) {
	mbL := make([][]byte, 0, 4096)
	for i := 0; i < 4096; i++ {
		bf := Malloc(int(rand.Int63()%4080) + 16)
		mbL = append(mbL, bf)
	}
	fmt.Println(MemPoolStat())
	fmt.Println("## to free")
	for i := range mbL {
		Free(mbL[i])
	}
	fmt.Println("## free end")
	fmt.Println(MemPoolStat())
	MemPoolGC()

	mbL = mbL[:0]
	for i := 0; i < 1024; i++ {
		mb := Malloc(int(rand.Int63()%2032) + 16)
		mbL = append(mbL, mb)
	}
	for i := range mbL {
		Free(mbL[i])
	}

	MemPoolGC()
	fmt.Println("## gc end")
	fmt.Println(MemPoolStat())
}
func BenchmarkMempool(b *testing.B) {
	cases := []struct {
		name string
		N    int
	}{
		{"N-0.2k", 256},
		{"N-1k", 1024 * 1},
		{"N-4k", 1024 * 4},
		{"N-8k", 1024 * 8},
		{"N-16k", 1024 * 16},
		{"N-32k", 1024 * 32},
		{"N-64k", 1024 * 64},
	}
	for _, c := range cases {
		b.Run(c.name, func(b *testing.B) {
			mbL := make([][]byte, 4096)
			for i := 0; i < b.N; i++ {
				for i := 0; i < len(mbL); i++ {
					mb := Malloc(c.N)
					mbL[i] = mb
					mb[len(mb)-1] = 'a'
				}
				for i := range mbL {
					Free(mbL[i])
				}
			}
		})
	}
}
func BenchmarkMake(b *testing.B) {
	cases := []struct {
		name string
		N    int
	}{
		{"N-0.2k", 256},
		{"N-1k", 1024 * 1},
		{"N-4k", 1024 * 4},
		{"N-8k", 1024 * 8},
		{"N-16k", 1024 * 16},
		{"N-32k", 1024 * 32},
		{"N-64k", 1024 * 64},
	}
	for _, c := range cases {
		b.Run(c.name, func(b *testing.B) {
			mbL := make([][]byte, 4096)
			for i := 0; i < b.N; i++ {
				for i := 0; i < len(mbL); i++ {
					if true { // in heap
						mb := make([]byte, c.N)
						mbL[i] = mb
						mb[len(mb)-1] = 'a'
					} else { // in stack
						mb := make([]byte, 2098)
						mb[len(mb)-1] = 'a'
					}
				}
				for i := range mbL {
					mbL[i] = nil
				}
			}
		})
	}
}
func BenchmarkMSyncPool(b *testing.B) {
	cases := []struct {
		name string
		N    int
		I    int
	}{
		{"N-0.2k", 256, 0},
		{"N-1k", 1024 * 1, 1},
		{"N-4k", 1024 * 4, 2},
		{"N-8k", 1024 * 8, 3},
		{"N-16k", 1024 * 16, 4},
		{"N-32k", 1024 * 32, 5},
		{"N-64k", 1024 * 64, 6},
	}
	var spArr [7]sync.Pool
	spArr[0].New = func() any { return make([]byte, 256) }
	spArr[1].New = func() any { return make([]byte, 1024) }
	spArr[2].New = func() any { return make([]byte, 1024*4) }
	spArr[3].New = func() any { return make([]byte, 1024*8) }
	spArr[4].New = func() any { return make([]byte, 1024*16) }
	spArr[5].New = func() any { return make([]byte, 1024*32) }
	spArr[6].New = func() any { return make([]byte, 1024*64) }
	for _, c := range cases {
		b.Run(c.name, func(b *testing.B) {
			mbL := make([][]byte, 4096)
			for i := 0; i < b.N; i++ {
				for i := 0; i < len(mbL); i++ {
					mb := spArr[c.I].Get().([]byte)
					mbL[i] = mb
					mb[len(mb)-1] = 'a'
				}
				for i := range mbL {
					spArr[c.I].Put(mbL[i])
				}
			}
		})
	}
}
func BenchmarkMRingBuffer(b *testing.B) {
	cases := []struct {
		name string
		N    int
		I    int
	}{
		{"N-0.2k", 256, 0},
		{"N-1k", 1024 * 1, 1},
		{"N-4k", 1024 * 4, 2},
		{"N-8k", 1024 * 8, 3},
		{"N-16k", 1024 * 16, 4},
		{"N-32k", 1024 * 32, 5},
		{"N-64k", 1024 * 64, 6},
	}
	var spArr [7]*RingBuffer[[]byte]
	for i := 0; i < len(spArr); i++ {
		spArr[i] = NewRingBuffer[[]byte](4096)
	}
	for _, c := range cases {
		b.Run(c.name, func(b *testing.B) {
			mbL := make([][]byte, 4096)
			for i := 0; i < b.N; i++ {
				for i := 0; i < len(mbL); i++ {
					if spArr[c.I].IsEmpty() {
						spArr[c.I].PushBack(make([]byte, c.N))
					}
					mb, _ := spArr[c.I].PopFront()
					mbL[i] = mb
					mb[len(mb)-1] = 'a'
				}
				for i := range mbL {
					spArr[c.I].PushBack(mbL[i])
				}
			}
		})
	}
}
func BenchmarkMpool(b *testing.B) {
	cases := []struct {
		name string
		N    int
	}{
		{"N-0.2k", 256},
		{"N-1k", 1024 * 1},
		{"N-4k", 1024 * 4},
		{"N-8k", 1024 * 8},
		{"N-16k", 1024 * 16},
		{"N-32k", 1024 * 32},
		{"N-64k", 1024 * 64},
	}
	for _, c := range cases {
		b.Run(c.name, func(b *testing.B) {
			mbL := make([][]byte, 4096)
			for i := 0; i < b.N; i++ {
				for i := 0; i < len(mbL); i++ {
					mb := AMalloc(c.N)
					mbL[i] = mb
					mb[len(mb)-1] = 'a'
				}
				for i := range mbL {
					AFree(mbL[i])
				}
			}
		})
	}
}
