package main

import (
	"fmt"
	"syscall"
	"unsafe"
)

type Boy struct {
	Name string
	Age  int
}

func (b *Boy) Say() {
	fmt.Printf("my name is %s, age %d\n", b.Name, b.Age)
}

func main() {
	var ev syscall.EpollEvent
	var boys []*Boy
	boys = append(boys, new(Boy))
	boys[0].Age = 25
	boys[0].Name = "Jim"

	boy := boys[0]

	*(**Boy)(unsafe.Pointer(&ev.Fd)) = boy

	boy2 := *(**Boy)(unsafe.Pointer(&ev.Fd))
	boy2.Say()
	boys[0].Age = 30
	boy2.Say()
}
