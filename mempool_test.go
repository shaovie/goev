package goev

import (
	"fmt"
	"math/rand"
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
		N    int64
	}{
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
					mb := Malloc(int(rand.Int63()%(c.N-16)) + 16)
					//mb := Malloc(2048 + int(rand.Int63() % 256))
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
		N    int64
	}{
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
						mb := make([]byte, int(rand.Int63()%(c.N-16))+16)
						//mb := make([]byte, 2048 + int(rand.Int63()%256))
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
