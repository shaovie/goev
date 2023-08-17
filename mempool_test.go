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
	for _, v := range mbL {
		Free(v)
	}
	fmt.Println(MemPoolStat())
}
