package goev

import (
    "sync"
)

// 经过测试, 性能不稳定, 线程增加后arr的锁带来的影响会成倍增加

// Thread safe array + map
// Refer to test/mutex_arr_vs_map.go
// Indexes with a small range use array indexing, while indexes with a large range use a map.
// All threads are secure.
//
// Saving nil requires attention to semantics, as when loading fails to find a value in the map,
// it will also return nil.
type MtxArrayMap[T any] struct {
    notFoundRet T

    arrSize int
    arr []T
    arrMtx sync.RWMutex

    sMap sync.Map
}

func NewMtxArrayMap[T any](arrSize int, notFoundRet T) *MtxArrayMap[T] {
    if arrSize < 1 {
        return nil
    }
    return &MtxArrayMap[T] {
        arrSize: arrSize,
        arr: make([]T, arrSize),
        notFoundRet: notFoundRet,
    }
}

func (am *MtxArrayMap[T]) Load(i int) T {
    if i < am.arrSize {
        am.arrMtx.RLock()
        defer am.arrMtx.RUnlock()
        return am.arr[i]
    }
    if v, ok := am.sMap.Load(i); ok {
        return v.(T)
    }
    return am.notFoundRet
}
func (am *MtxArrayMap[T]) Store(i int, v T) {
    if i < am.arrSize {
        am.arrMtx.Lock()
        am.arr[i] = v
        am.arrMtx.Unlock()
        return
    }
    am.sMap.Store(i, v)
}
func (am *MtxArrayMap[T]) Delete(i int) {
    if i < am.arrSize {
        am.arrMtx.Lock()
        am.arr[i] = am.notFoundRet
        am.arrMtx.Unlock()
        return
    }
    am.sMap.Delete(i)
}
