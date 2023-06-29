// Autor cuisw. 2023
package goev

import (
	"errors"
	"runtime"
)

type Reactor struct {
	evPoll *evPoll
}

func NewReactor(opts ...Option) (*Reactor, error) {
	setOptions(opts...)
	if evOptions.evPollThreadNum < 1 || evOptions.evPollThreadNum > runtime.NumCPU()*2 {
		return nil, errors.New("options: EvPollThreadNum MUST > 0 && <= cpu*2")
	}
	r := &Reactor{
		evPoll: new(evPoll),
	}
	return r, nil
}

func (r *Reactor) Open() error {
	return r.evPoll.open(evOptions.evPollThreadNum, evOptions.evPollSize)
}

func (r *Reactor) AddEvHandler(h EvHandler, fd, events int) error {
	return r.evPoll.add(fd, events, h)
}

func (r *Reactor) RemoveEvHandler(fd int) error {
	return r.evPoll.remove(fd)
}

func (r *Reactor) Run() error {
	return r.evPoll.run()
}
