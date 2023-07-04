package main

import (
	"errors"
	"fmt"
	"syscall"
)

func main() {
	buf := make([]byte, 16)
	syscall.SetNonblock(0, true)

	n, err := syscall.Read(0, buf)
	if errors.Is(err, syscall.EAGAIN) {
		fmt.Println("is", n, err.Error())
	}
	if errors.Is(err, syscall.EWOULDBLOCK) {
		fmt.Println("is wouldblock", n, err.Error())
	}
	if err == syscall.EAGAIN {
		fmt.Println("=", err.Error())
	}
	if err == syscall.EWOULDBLOCK {
		fmt.Println("= wouldblock", err.Error())
	}
	if err.(syscall.Errno) == syscall.EWOULDBLOCK {
		fmt.Println("assert == wouldblock", err.Error())
	}
}
