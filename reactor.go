// Autor cuisw. 2023.06
package goev

import (
	"errors"
	"fmt"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
)

type Reactor struct {
	noCopy

	evPollLockOSThread bool
	evPollNum          int
	evPolls            []evPoll
	timerIdx           atomic.Int64
}

func NewReactor(opts ...Option) (*Reactor, error) {
	setOptions(opts...)
	if evOptions.evPollNum < 1 {
		panic("options: EvPollThreadNum MUST > 0")
	}
	r := &Reactor{
		evPollLockOSThread: evOptions.evPollLockOSThread,
		evPollNum:          evOptions.evPollNum,
		evPolls:            make([]evPoll, evOptions.evPollNum),
	}
	var timer timer
	if evOptions.noTimer == false {
		timer = newTimer4Heap(evOptions.timerHeapInitSize)
	}
	for i := 0; i < r.evPollNum; i++ {
		if err := r.evPolls[i].open(evOptions.evReadyNum, evOptions.evPollSharedBuffSize,
			evOptions.evDataArrSize, timer); err != nil {
			return nil, err
		}
	}
	return r, nil
}

// Translate: "The same EvHandler is not allowed to be registered more than once in the Reactor."
func (r *Reactor) AddEvHandler(eh EvHandler, fd int, events uint32) error {
	if fd < 0 || eh == nil {
		return errors.New("AddEvHandler: invalid params")
	}
	i := 0
	if r.evPollNum > 1 {
        // fd is a self-incrementing and cyclic integer, can be allocated through round-robin distribution.
		i = fd % r.evPollNum
	}
	return r.evPolls[i].add(fd, events, eh)
}

func (r *Reactor) RemoveEvHandler(eh EvHandler, fd int) error {
	if eh == nil || fd < 0 {
		return errors.New("invalid EvHandler or fd")
	}
	if ep := eh.getEvPoll(); ep != nil {
		return ep.remove(fd)
	}
	return errors.New("ev handler not add")
}

// delay, interval are both relative time measurements with millisecond accuracy, for example, delay=5msec.
func (r *Reactor) SchedueTimer(eh EvHandler, delay, interval int64) error {
	i := 0
	if r.evPollNum > 1 {
		if ep := eh.getEvPoll(); ep != nil {
			return ep.scheduleTimer(eh, delay, interval)
		} else {
			i = int(r.timerIdx.Add(1) % int64(r.evPollNum))
		}
	}
	return r.evPolls[i].scheduleTimer(eh, delay, interval)
}
func (r *Reactor) Run() error {
	var wg sync.WaitGroup
	var errS []string
	var errSMtx sync.Mutex
	for i := 0; i < r.evPollNum; i++ {
		wg.Add(1)
		go func(j int) {
			if r.evPollLockOSThread == true {
				// Refer to go doc runtime.LockOSThread
				// LockOSThread will bind the current goroutine to the current OS thread T,
				// preventing other goroutines from being scheduled onto this thread T
				runtime.LockOSThread()
			}
			err := r.evPolls[j].run(&wg)
			errSMtx.Lock()
			errS = append(errS, fmt.Sprintf("epoll#%d err: %s", j, err.Error()))
			errSMtx.Unlock()
		}(i)
	}
	wg.Wait()
	return errors.New(strings.Join(errS, "; "))
}
