package main

import (
	"fmt"
    "time"
    "sync"
    "runtime"
    "math/rand"
)
type EpollEvent struct {
	Events uint32
	Fd  int32
	Pad int32
}
// 加了 LockOSThread之后,arr性能降了一个数量级, 看来随着程序运行时间长了动态增长线程后arr就不好使了,不稳定
// LockOSThread 会强制新建一个线程
var (
    userLockOSThread = true
    loopN = 200000
    arrSize = 8192 // 对性能影响不是线性的
    arr []*EpollEvent
    arrMtx sync.RWMutex

    m sync.Map
)
func arrGet(i int) *EpollEvent {
    arrMtx.RLock()
    defer arrMtx.RUnlock()
    return arr[i]
}
func arrSet(i int, v *EpollEvent) {
    arrMtx.Lock()
    arr[i] = v
    arrMtx.Unlock()
}
func arrMutexR(wg *sync.WaitGroup) {
    if userLockOSThread {
        runtime.LockOSThread()
    }
    for i := 0; i < loopN; i++ {
        j := int(rand.Int63()) % arrSize
        if v := arrGet(j); v != nil {
            if v.Fd == 0 {
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
            if v.(*EpollEvent).Fd == 0 {
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
    arr = make([]*EpollEvent, arrSize)
    var wg sync.WaitGroup
    wg.Add(1)
    begin := time.Now()
    go arrMutexW(&wg)
    for i := 0; i < 5; i++ {
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
    for i := 0; i < 5; i++ {
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
