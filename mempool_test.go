package goev

import (
	"fmt"
	"math/rand"
	"testing"
)

func TestMempool(t *testing.T) {
	mbL := make([]MBuff, 0, 4096)
	for i := 0; i < 4097; i++ {
		mb := Malloc(int(rand.Int63() % 4096))
		if mb.sg == nil {
			fmt.Println("err: sg == nil")
			return
		}
		mbL = append(mbL, mb)
	}
	fmt.Println(MemPoolStat())
	fmt.Println("--------------------------------------")
	for i := range mbL {
		Free(mbL[i])
	}
	fmt.Println(MemPoolStat())
}
func BenchmarkMempool(b *testing.B) {
	mbL := make([]MBuff, 4096)
	for i := 0; i < b.N; i++ {
		for i := 0; i < 4096; i++ {
			mb := Malloc(int(rand.Int63() % 4096))
			mbL[i] = mb
		}
		for i := range mbL {
			Free(mbL[i])
		}
	}
}
func BenchmarkMake(b *testing.B) {
	mbL := make([][]byte, 4096)
	for i := 0; i < b.N; i++ {
		for i := 0; i < 4096; i++ {
			mb := make([]byte, int(rand.Int63()%4096))
			mbL[i] = mb
		}
		for i := range mbL {
			mbL[i] = nil
		}
	}
}
