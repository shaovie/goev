package main

import (
	"fmt"
	"strconv"
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

// 测试用unsafe.Pointer的方式对转化后的内存区域进行操作, 会不会影响旁边的内存
func main() {
	var ev syscall.EpollEvent
	fmt.Printf("EpollEvent sizeof %d\n", unsafe.Sizeof(ev))
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

	// begin
	evL := make([]syscall.EpollEvent, 1000)
	boyL := make([]Boy, 1000)
	for i := 0; i < len(evL); i++ {
		ev := &evL[i]
		ev.Events = uint32(i)
		boy := &boyL[i]
		*(**Boy)(unsafe.Pointer(&ev.Fd)) = boy
	}
	for i := 0; i < len(boyL); i++ {
		boyL[i].Age = i
		boyL[i].Name = strconv.FormatInt(int64(i), 10)
	}
	for i := 0; i < len(boyL); i++ {
		boy := &boyL[i]
		if boy.Age != i {
			fmt.Printf("name %s, age %d\n", boy.Name, boy.Age)
		}
	}
	// modify Events
	for i := 0; i < len(evL); i++ {
		ev := &evL[i]
		ev.Events = uint32(i * 2)
		boy := *(**Boy)(unsafe.Pointer(&ev.Fd))
		if boy.Age != i {
			fmt.Printf("name %s, age %d\n", boy.Name, boy.Age)
		}
	}
	fmt.Printf("modified\n")
	for i := 0; i < len(evL); i++ {
		ev := &evL[i]
		if ev.Events != uint32(i*2) {
			fmt.Printf("events %d\n", ev.Events)
		}
		boy := *(**Boy)(unsafe.Pointer(&ev.Fd))
		if boy.Age != i {
			fmt.Printf("name %s, age %d\n", boy.Name, boy.Age)
		}
	}
}
