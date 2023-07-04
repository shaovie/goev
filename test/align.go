package main

import (
	"fmt"
	"time"
	"unsafe"
)

type EpollEvent struct {
	Events uint32
	Fd     int32
	Pad    int32
	// Fd    [8]byte // 2023.6.29 测试发现用[8]byte的内存方式还是会出现崩溃的问题
}

func main() {
	var ev EpollEvent
	fmt.Printf("EpollEvent sizeof %d\n", unsafe.Sizeof(ev))

	evMap := make(map[int]*EpollEvent)
	for {
		for i := 0; i < 10000; i++ {
			ev := new(EpollEvent)
			evMap[i] = ev
			*(*int64)(unsafe.Pointer(&ev.Fd)) = int64(i)
		}
		time.Sleep(time.Second * 1)
		for i := 0; i < 10000; i++ {
			ev := evMap[i]
			v := *(*int64)(unsafe.Pointer(&ev.Fd))
			if v != int64(i) {
				panic("crashed")
			}
			delete(evMap, i)
		}
	}
}
