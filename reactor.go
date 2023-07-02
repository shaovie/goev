// Autor cuisw. 2023
package goev

import (
    "fmt"
	"sync"
	"errors"
	"strings"
)

type Reactor struct {
    noCopy

    evPollSize int
	evPolls []evPoll
}

func NewReactor(opts ...Option) (*Reactor, error) {
	setOptions(opts...)
	if evOptions.evPollSize < 1 {
		return nil, errors.New("options: EvPollThreadNum MUST > 0")
	}
    r := &Reactor{
        evPollSize: evOptions.evPollSize,
        evPolls: make([]evPoll, evOptions.evPollSize),
    }
    for i := 0; i < r.evPollSize; i++ {
        if err := r.evPolls[i].open(evOptions.evReadySize, evOptions.evDataArrSize); err != nil {
            return nil, err
        }
    }
	return r, nil
}
func (r *Reactor) AddEvHandler(h EvHandler, fd int, events uint32) error {
    if fd < 0 || h == nil {
        return errors.New("AddEvHandler: invalid params")
    }
    i := fd % r.evPollSize
	return r.evPolls[i].add(fd, events, h)
}
// fd 一定是reactor内部构造的, 不能自己构造
func (r *Reactor) RemoveEvHandler(fd *Fd) error {
    if fd == nil || fd.v < 0 {
		return errors.New("invalid fd")
    }
    i := fd.v % r.evPollSize
	return r.evPolls[i].remove(fd)
}
func (r *Reactor) Run() error {
    var wg sync.WaitGroup
    var errS []string
    var errSMtx sync.Mutex
    for i := 0; i < r.evPollSize; i++ {
        wg.Add(1)
        go func(j int) {
            err := r.evPolls[j].run(&wg)
            errSMtx.Lock()
            errS = append(errS, fmt.Sprintf("epoll#%d err: %s", j, err.Error()))
            errSMtx.Unlock()
        }(i)
    }
    wg.Wait()
    return errors.New(strings.Join(errS, ";"))
}
