package goev

import (
	"fmt"
	"testing"
)

func TestLog(t *testing.T) {
	fmt.Println("hello boy")
	log, _ := NewLog("logs")
	for i := 0; i < 100; i++ {
		log.Debug("hello %s %d", "debug", i)
		log.Rinfo("hello %s %d", "rinfo", i)
		log.Error("hello %s %d", "error", i)
		log.Fatal("hello %s %d", "fatal", i)
		log.Warning("hello %s %d", "warning", i)
	}
	log1, _ := NewLog("")
	for i := 0; i < 2; i++ {
		log1.Debug("hello %s %d", "debug", i)
		log1.Error("hello %s %d", "error", i)
		log1.Warning("hello %s %d", "warning", i)
		log1.Fatal("hello %s %d", "fatal", i)
		log1.Rinfo("hello %s %d", "rinfo", i)
	}
}
