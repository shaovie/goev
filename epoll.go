package goev

import (
	"sync"
	"time"
	"errors"
	"runtime"
	"syscall"
	"sync/atomic"
)

// Use arrays for low range, and use maps for high range.
type evHandlerMap struct {
	arrSize int
	arr     []atomic.Pointer[EvHandler]
	sMap sync.Map
}
func (m *evHandlerMap) Load(i int) EvHandler {
	if i < m.arrSize {
        e := m.arr[i].Load()
        return *e
	}
	if v, ok := m.sMap.Load(i); ok {
		return v.(EvHandler)
	}
	return nil
}
func (m *evHandlerMap) Store(i int, v EvHandler) {
	if i < m.arrSize {
		m.arr[i].Store(&v)
		return
	}
	m.sMap.Store(i, v)
}
func (m *evHandlerMap) Delete(i int) {
	if i < m.arrSize {
		m.arr[i].Store(nil)
		return
	}
	m.sMap.Delete(i)
}

// evPoll
type evPoll struct {
	efd int // epoll fd

	evReadyNum int // epoll_wait一次轮询获取固定数量准备好的I/O事件, 此参数有利于线程处理的敏捷性

	evHandlerMap *evHandlerMap // Refer to https://zhuanlan.zhihu.com/p/640712548
	timer        timer
	evPollWackup Notifier
}

func (ep *evPoll) open(evReadyNum, evDataArrSize int, timer timer) error {
	if evReadyNum < 1 {
		return errors.New("EvReadyNum < 1")
	}
	efd, err := syscall.EpollCreate1(syscall.EPOLL_CLOEXEC)
	if err != nil {
		return errors.New("syscall epoll_create1: " + err.Error())
	}
	ep.evReadyNum = evReadyNum
	ep.efd = efd
	ep.evHandlerMap = &evHandlerMap{
		arrSize: evDataArrSize,
		arr:     make([]atomic.Pointer[EvHandler], evDataArrSize),
	}
	ep.timer = timer

	// Must be placed last
	ep.evPollWackup, err = newNotify(ep)
	if err != nil {
		return err
	}
	// process max fds
	// show using `ulimit -Hn`
	// $GOROOT/src/os/rlimit.go Go had raise the limit to 'Max Hard Limit'
	return nil
}
func (ep *evPoll) add(fd int, events uint32, eh EvHandler) error {
	eh.setEvPoll(ep)

	ev := syscall.EpollEvent{
		Events: events,
		Fd:     int32(fd),
	}
	ep.evHandlerMap.Store(fd, eh)

	if err := syscall.EpollCtl(ep.efd, syscall.EPOLL_CTL_ADD, fd, &ev); err != nil {
		return errors.New("epoll_ctl add: " + err.Error())
	}
	return nil
}
func (ep *evPoll) remove(fd int) error {
	// The event argument is ignored and can be NULL (but see `man 2 epoll_ctl` BUGS)
	// kernel versions > 2.6.9
	ep.evHandlerMap.Delete(fd)
	if err := syscall.EpollCtl(ep.efd, syscall.EPOLL_CTL_DEL, fd, nil); err != nil {
		return errors.New("epoll_ctl del: " + err.Error())
	}
	return nil
}
func (ep *evPoll) scheduleTimer(eh EvHandler, delay, interval int64) (err error) {
	err = ep.timer.schedule(eh, delay, interval)
	ep.evPollWackup.Notify()
	return
}
func (ep *evPoll) run(wg *sync.WaitGroup) error {
	if wg != nil {
		defer wg.Done()
	}

	// Refer to go doc runtime.LockOSThread
	// LockOSThread will bind the current goroutine to the current OS thread T,
	// preventing other goroutines from being scheduled onto this thread T
	runtime.LockOSThread()

	var nfds, i, msec int
	var err error
	var now int64
	msec = -1
	events := make([]syscall.EpollEvent, ep.evReadyNum) // NOT make(x, len, cap)
	for {
		nfds, err = syscall.EpollWait(ep.efd, events, msec)
		now = time.Now().UnixMilli()
		msec = int(ep.timer.handleExpired(now))
		if nfds > 0 {
			for i = 0; i < nfds; i++ {
				ev := &events[i]
                fd := int(ev.Fd)
				eh := ep.evHandlerMap.Load(fd)
				if eh == nil {
					continue // TODO add evOptions.debug? panic("evHandlerMap not found")
				}
				// EPOLLHUP refer to man 2 epoll_ctl
				if ev.Events&(syscall.EPOLLHUP|syscall.EPOLLERR) != 0 {
					ep.remove(fd) // MUST before OnClose()
					eh.OnClose(fd)
					continue
				}
				if ev.Events&(syscall.EPOLLOUT) != 0 { // MUST before EPOLLIN (e.g. connect)
					if eh.OnWrite(fd, now) == false {
						ep.remove(fd) // MUST before OnClose()
						eh.OnClose(fd)
						continue
					}
				}
				if ev.Events&(syscall.EPOLLIN) != 0 {
					if eh.OnRead(fd, now) == false {
						ep.remove(fd) // MUST before OnClose()
						eh.OnClose(fd)
						continue
					}
				}
			} // end of `for i < nfds'
		} else if nfds == 0 { // timeout
			continue
		} else if err != nil && err != syscall.EINTR { // nfds < 0
			return errors.New("syscall epoll_wait: " + err.Error())
		}
	}
	return nil
}
