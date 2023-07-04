package main

import (
	"fmt"
	"math/rand"
	"runtime"
	"sync"
	"sync/atomic"
	"time"
)

type EpollEvent struct {
	fd  int
	fd2 *int64
}

var (
	userLockOSThread = true
	loopN            = 500000
	arrSize          = 8192 // 对性能影响不是线性的
	arr              []atomic.Pointer[EpollEvent]

	m sync.Map
)

func arrGet(i int) *EpollEvent {
	return arr[i].Load()
}
func arrSet(i int, v *EpollEvent) {
	arr[i].Store(v)
}
func arrMutexR(wg *sync.WaitGroup) {
	if userLockOSThread {
		runtime.LockOSThread()
	}
	for i := 0; i < loopN; i++ {
		j := int(rand.Int63()) % arrSize
		if v := arrGet(j); v != nil {
			if v.fd == j {
				_ = rand.Int63()
			} else {
				panic("arr val")
			}
		}
	}
	wg.Done()
}
func arrMutexW(wg *sync.WaitGroup) {
	if userLockOSThread {
		runtime.LockOSThread()
	}
	for i := 0; i < loopN; i++ {
		j := int(rand.Int63()) % arrSize
		arrSet(j, &EpollEvent{fd: j})
	}
	wg.Done()
}
func mapR(wg *sync.WaitGroup) {
	if userLockOSThread {
		runtime.LockOSThread()
	}
	for i := 0; i < loopN; i++ {
		j := int(rand.Int63()) % arrSize
		if v, ok := m.Load(j); ok {
			if v.(*EpollEvent).fd == j {
				_ = rand.Int63()
			} else {
				panic("map val")
			}
		}
	}
	wg.Done()
}
func mapW(wg *sync.WaitGroup) {
	if userLockOSThread {
		runtime.LockOSThread()
	}
	for i := 0; i < loopN; i++ {
		j := int(rand.Int63()) % arrSize
		m.Store(j, &EpollEvent{fd: j})
	}
	wg.Done()
}
func arrTest() {
	arr = make([]atomic.Pointer[EpollEvent], arrSize)
	/*
	   for i := 0; i < arrSize; i++ {
	       arr[i] = new(atomic.Pointer[EpollEvent])
	   }*/
	var wg sync.WaitGroup
	wg.Add(1)
	begin := time.Now()
	go arrMutexW(&wg)
	for i := 0; i < 3; i++ {
		wg.Add(1)
		go arrMutexR(&wg)
	}
	wg.Wait()
	diff := time.Now().Sub(begin).Milliseconds()
	fmt.Println("arr", diff)
}
func mapTest() {
	var wg sync.WaitGroup
	wg.Add(1)
	begin := time.Now()
	go mapW(&wg)
	for i := 0; i < 3; i++ {
		wg.Add(1)
		go mapR(&wg)
	}
	wg.Wait()
	diff := time.Now().Sub(begin).Milliseconds()
	fmt.Println("map", diff)
}
func main() {
	at1 := make([]atomic.Pointer[EpollEvent], 5)
	at1[0].Store(&EpollEvent{fd: 0})
	at1[1].Store(&EpollEvent{fd: 1})
	at1[2].Store(&EpollEvent{fd: 2})
	at11 := at1[1].Load()
	fmt.Println(at11.fd)
	mapTest()
	arrTest()
}
