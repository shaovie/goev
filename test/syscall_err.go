package main

import (
    "fmt"
	"errors"
	"syscall"
)

func main() {
    buf := make([]byte, 16)
    n, err := syscall.Read(-1, buf)
    fmt.Println(n, err.Error())
    if errors.Is(err, syscall.EBADF) {
        fmt.Println(err.Error())
    }
    if err == syscall.EBADF {
        fmt.Println(err.Error())
    }
}
