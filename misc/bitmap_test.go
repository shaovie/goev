package goev

import (
	"fmt"
	"math/rand"
	"testing"
)

func TestBitmap(t *testing.T) {
	fmt.Println("hello boy")
	b := NewBitMap(100)
	set := make([]int64, 0, 100)
	for i := int64(0); i < 10; i++ {
		v := rand.Int63() % 100
		fmt.Println("set", v)
		set = append(set, v)
		b.Set(v)
	}
	for i := int64(0); i < 100; i++ {
		if b.IsSet(i) {
			fmt.Println("is set", i)
		}
	}
	for _, v := range set {
		if b.IsSet(v) == false {
			panic("error")
		}
	}
}
