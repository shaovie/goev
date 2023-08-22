package goev

// Autor cuisw. 2023.07

import (
	"errors"
	"fmt"
	"runtime"
	"strings"
	"sync"
)

// Reactor provides an I/O event-driven event handling model, where multiple epoll processes
// can be specified internally. The file descriptors (fd) between multiple Reactors can be
// bound to each other, enabling concurrent processing in multiple threads.
type Reactor struct {
	noCopy

	evPollLockOSThread bool
	evPollNum          int
	evPolls            []evPoll
}

// NewReactor return an instance
func NewReactor(opts ...Option) (*Reactor, error) {
	evOptions := setOptions(opts...)
	if evOptions.evPollNum < 1 {
		panic("options: EvPollThreadNum MUST > 0")
	}
	r := &Reactor{
		evPollLockOSThread: evOptions.evPollLockOSThread,
		evPollNum:          evOptions.evPollNum,
		evPolls:            make([]evPoll, evOptions.evPollNum),
	}
	for i := 0; i < r.evPollNum; i++ {
		r.evPolls[i].id = i
		timer := newTimer4Heap(evOptions.timerHeapInitSize)
		if err := r.evPolls[i].open(evOptions.evFdMaxSize, timer,
			evOptions.evPollReadBuffSize, evOptions.evPollWriteBuffSize); err != nil {
			return nil, err
		}
		r.evPolls[i].add(timer.timerfd(), EvIn, timer)
	}
	return r, nil
}

// AddEvHandler can register a file descriptor (fd) and its corresponding handler object into the Reactor.
// If multiple evPool instances are specified internally, the fd will be rotated to the designated
// evPool instance based on fd % idx.
func (r *Reactor) AddEvHandler(eh EvHandler, fd int, events uint32) error {
	if fd < 1 || eh == nil { // NOTE fd must > 0
		return errors.New("AddEvHandler: invalid params")
	}
	i := 0
	if r.evPollNum > 1 {
		// fd is a self-incrementing and cyclic integer, can be allocated through round-robin distribution.
		i = fd % r.evPollNum
	}
	eh.setReactor(r) // MUST before add
	return r.evPolls[i].add(fd, events, eh)
}

// AppendEvent appending events to EvHandler that has already been added to the poller
// via AddEvHandler (EvHandler value does not need to be provided here)
func (r *Reactor) AppendEvent(fd int, events uint32) error {
	if fd < 1 || events == 0 { // NOTE fd must > 0
		return errors.New("AppendEvent: invalid params")
	}
	i := 0
	if r.evPollNum > 1 {
		i = fd % r.evPollNum
	}
	return r.evPolls[i].append(fd, events)
}

// RemoveEvent removes the events registered for fd.
//
// By using EvAll, all associated events can be removed, and the EvHandler will be unbound from the evPoll
func (r *Reactor) RemoveEvent(fd int, events uint32) error {
	if fd < 1 || events == 0 { // NOTE fd must > 0
		return errors.New("RemoveEvent: invalid params")
	}
	i := 0
	if r.evPollNum > 1 {
		i = fd % r.evPollNum
	}
	return r.evPolls[i].remove(fd, events)
}

// InitPollSyncOpt called before Reactor.Run
func (r *Reactor) InitPollSyncOpt(typ int, val any) {
	for i := 0; i < r.evPollNum; i++ {
		r.evPolls[i].pollSyncOpt(typ, val)
	}
}

// PollSyncOpt xx
func (r *Reactor) PollSyncOpt(typ int, val any) {
	for i := 0; i < r.evPollNum; i++ {
		r.evPolls[i].pollSyncOpt(typ, val)
	}
}

// Run starts the multi-event evpolling to run.
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
			errS = append(errS, fmt.Sprintf("evPoll#%d err: %s", j, err.Error()))
			errSMtx.Unlock()
		}(i)
	}
	wg.Wait()

	if len(errS) == 0 {
		return nil
	}
	return errors.New(strings.Join(errS, "; "))
}
