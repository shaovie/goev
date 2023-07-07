package goev

import (
	"testing"
)

func TestLog(t *testing.T) {
	Info("hello boy")
	log, _ := NewLog("logs")
	for i := 0; i < 10; i++ {
		log.Debug("hello %s %d", "debug", i)
		log.Info("hello %s %d", "rinfo", i)
		log.Error("hello %s %d", "error", i)
		log.Fatal("hello %s %d", "fatal", i)
		log.Warn("hello %s %d", "warn", i)
	}
	for i := 0; i < 10; i++ {
		Debug("append %s %d", "debug", i)
		Info("append %s %d", "rinfo", i)
		Error("append %s %d", "error", i)
		Fatal("append %s %d", "fatal", i)
		Warn("append %s %d", "warn", i)
	}
	log1, _ := NewLog("")
	for i := 0; i < 2; i++ {
		log1.Debug("hello %s %d", "debug", i)
		log1.Error("hello %s %d", "error", i)
		log1.Warn("hello %s %d", "warn", i)
		log1.Fatal("hello %s %d", "fatal", i)
		log1.Info("hello %s %d", "rinfo", i)
	}
	for i := 0; i < 10; i++ {
		Debug("append %s %d", "debug", i)
		Info("append %s %d", "rinfo", i)
		Error("append %s %d", "error", i)
		Fatal("append %s %d", "fatal", i)
		Warn("append %s %d", "warn", i)
	}
}
