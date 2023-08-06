package goev

import (
	"fmt"
	"testing"
)

func TestRingBuffer(t *testing.T) {
	rb := NewRingBuffer[int](50)

	for i := 0; i < 100; i++ {
		rb.Push(i)
	}
	for i := 0; i < 50; i++ {
		rb.Pop()
	}
	t.Logf("is empty:%v\n", rb.IsEmpty())
	for i := 0; i < 50; i++ {
		rb.Push(i)
	}

	fmt.Println("len:", rb.Len())
	fmt.Println(rb.Pop()) // Output: 0
	fmt.Println(rb.Pop()) // Output: 1
	fmt.Println(rb.Pop()) // Output: 2

	for !rb.IsEmpty() {
		data, _ := rb.Pop()
		fmt.Println(data)
	}
	fmt.Println("len:", rb.Len())
}
