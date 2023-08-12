package goev

import (
	"fmt"
	"testing"
)

func TestRingBuffer(t *testing.T) {
	rb := NewRingBuffer[int](10)

	for i := 0; i < 10; i++ {
		rb.PushBack(i)
	}
	t.Logf("is full:%v\n", rb.IsFull())
	v, _ := rb.PopFront()
	rb.PushFront(v)
	t.Logf("is full:%v\n", rb.IsFull())

	for i := 0; i < 5; i++ {
		rb.PopFront()
	}
	for i := 0; i < 5; i++ {
		rb.PushBack(i)
	}
	t.Logf("is full:%v\n", rb.IsFull())

	fmt.Println("len:", rb.Len())
	fmt.Println(rb.PopFront()) // Output: 0
	fmt.Println(rb.PopFront()) // Output: 1

	for !rb.IsEmpty() {
		data, _ := rb.PopFront()
		fmt.Println(data)
	}
	fmt.Println("len:", rb.Len())
}
