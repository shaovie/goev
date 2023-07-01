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
    if err := r.open(evOptions.evPollThreadNum, evOptions.evPollSize, evOptions.evDataArrSize); err != nil {
        return nil, err
    }
	return r, nil
}

func (r *Reactor) open(evPollThreadNum, evPollSize, evDataArrSize int) error {
	return r.evPoll.open(evPollThreadNum, evPollSize, evDataArrSize)
}

func (r *Reactor) AddEvHandler(h EvHandler, fd int, events uint32) error {
    if fd < 0 || h == nil {
        return errors.New("AddEvHandler: invalid params")
    }
	return r.evPoll.add(fd, events, h)
}
func (r *Reactor) ModifyEvHandler(h EvHandler, fd *Fd, events uint32) error {
    if fd == nil || fd.v < 0 || h == nil {
        return errors.New("ModifyEvHandler: invalid params")
    }
	return r.evPoll.modify(fd, events, h)
}
// fd 一定是reactor内部构造的, 不能自己构造
func (r *Reactor) RemoveEvHandler(fd *Fd) error {
    if fd == nil || fd.v < 0 {
		return errors.New("invalid fd")
    }
	return r.evPoll.remove(fd)
}

func (r *Reactor) Run() error {
	return r.evPoll.run()
}
