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
	arr     []atomic.Pointer[T]

	sMap sync.Map
}

// NewArrayMapUnion return an instance
//
// *T only Pointer
func NewArrayMapUnion[T any](arrSize int) *ArrayMapUnion[T] {
	if arrSize < 1 {
		panic("NewArrayMapUnion arrSize < 1")
	}
	amu := &ArrayMapUnion[T]{
		arrSize: arrSize,
		arr:     make([]atomic.Pointer[T], arrSize),
	}
	return amu
}

// Load returns the value stored in the array/map for a key, or nil if no
func (am *ArrayMapUnion[T]) Load(i int) *T {
	if i < am.arrSize {
		return am.arr[i].Load()
	}
	if v, ok := am.sMap.Load(i); ok {
		return v.(*T)
	}
	return nil
}

// Store sets the value for a key
func (am *ArrayMapUnion[T]) Store(i int, v *T) {
	if i < am.arrSize {
		am.arr[i].Store(v)
		return
	}
	am.sMap.Store(i, v)
}

// Delete deletes the value for a key
func (am *ArrayMapUnion[T]) Delete(i int) {
	if i < am.arrSize {
		am.arr[i].Store(nil)
		return
	}
	am.sMap.Delete(i)
}
