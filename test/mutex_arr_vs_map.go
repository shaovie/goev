package main

import (
	"fmt"
    "time"
    "sync"
    "sync/atomic"
    "runtime"
    "math/rand"
)
type EpollEvent struct {
    fd int
    fd2 *int64 
}
type ArrData struct {
    v atomic.Pointer[EpollEvent]
}
var (
    userLockOSThread = true
    loopN = 500000
    arrSize = 8192 // 对性能影响不是线性的
    arr []*atomic.Pointer[EpollEvent]

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
            if v.fd == 0 {
                _ = rand.Int63()
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
        arrSet(j, new(EpollEvent))
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
            if v.(*EpollEvent).fd == 0 {
                _ = rand.Int63()
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
        m.Store(j, new(EpollEvent))
    }
    wg.Done()
}
func arrTest() {
    arr = make([]*atomic.Pointer[EpollEvent], arrSize)
    for i := 0; i < arrSize; i++ {
        arr[i] = new(atomic.Pointer[EpollEvent])
    }
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
    mapTest()
    arrTest()
}
