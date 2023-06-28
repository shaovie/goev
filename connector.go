package goev

import (
	"syscall"
	"time"
)

type connnector struct {
	NullEvHandler
	fd               int
	events           int
	newEvHanlderFunc func() EvHandler
	reactor          *Reactor
}

func NewConnector(opts ...Option) (*connnector, error) {
	setOptions(opts...)
	a := &connnector{
		fd: -1,
	}
	return a, nil
}

func (c *connnector) Open(r *Reactor, newEvHanlderFunc func() EvHandler, events int) error {
	c.reactor = r
	c.events = events
	c.newEvHanlderFunc = newEvHanlderFunc
	return nil
}

// addr format 192.168.0.1:8080
func (c *connnector) Connect(addr string, timeout time.Duration) {
	fd, err := syscall.Socket(syscall.AF_INET, syscall.SOCK_STREAM, 0)
	_, _ = fd, err
}
