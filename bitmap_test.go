package goev

import (
	"fmt"
	"math/rand"
	"testing"
)

func TestBitmap(t *testing.T) {
	fmt.Println("hello boy")
	b := NewBitMap(62)
	set := make([]int, 0, 62)
	for i := 0; i < 10; i++ {
		v := rand.Int63() % 62
		fmt.Println("set", v)
		set = append(set, int(v))
		b.Set(int(v))
	}
	for i := 0; i < 62; i++ {
		if b.IsSet(i) {
			fmt.Println("is set", i)
		}
	}
	for _, v := range set {
		if b.IsSet(v) == false {
			panic("error")
		}
	}
	a := NewBitMap(62)
	for i := 0; i < 61; i++ {
		a.Set(i)
	}
	fmt.Println("first unset=", a.FirstUnSet())
}
