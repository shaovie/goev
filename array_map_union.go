package goev

import (
	"sync"
	"sync/atomic"
)

// ArrayMapUnion  is a composite data structure that includes an array and a map.
// Indexes with a small range use array indexing, while indexes with a large range use a map.
// It's thread safe atomic array + map
//
// Refer to test/mutex_arr_vs_map.go
type ArrayMapUnion[T any] struct {
	arrSize int
	arr     []atomic.Pointer[T] // TODO 如果针对fd, 这里应该可以不用atomic, 直接保存value

	// sync.Map is not suitable for use in evpoll as it is write-only, without read support
	sMap   map[int]*T
	mapMtx sync.Mutex
}

// NewArrayMapUnion return an instance
//
// *T only Pointer
func NewArrayMapUnion[T any](arrSize int) *ArrayMapUnion[T] {
	if arrSize < 1 {
		panic("NewArrayMapUnion arrSize < 1")
	}
	mapPreSize := arrSize / 3 // 1/4
	if mapPreSize < 1 {
		mapPreSize = 128
	}
	amu := &ArrayMapUnion[T]{
		arrSize: arrSize,
		arr:     make([]atomic.Pointer[T], arrSize),
		sMap:    make(map[int]*T, mapPreSize),
	}
	return amu
}

// Load returns the value stored in the array/map for a key, or nil if no
func (am *ArrayMapUnion[T]) Load(i int) *T {
	if i < am.arrSize {
		return am.arr[i].Load()
	}
	am.mapMtx.Lock()
	if v, ok := am.sMap[i]; ok {
		am.mapMtx.Unlock()
		return v
	}
	am.mapMtx.Unlock()
	return nil
}

// Store sets the value for a key
func (am *ArrayMapUnion[T]) Store(i int, v *T) {
	if i < am.arrSize {
		am.arr[i].Store(v)
		return
	}
	am.mapMtx.Lock()
	am.sMap[i] = v
	am.mapMtx.Unlock()
}

// Delete deletes the value for a key
func (am *ArrayMapUnion[T]) Delete(i int) {
	if i < am.arrSize {
		am.arr[i].Store(nil)
		return
	}
	am.mapMtx.Lock()
	delete(am.sMap, i)
	am.mapMtx.Unlock()
}
