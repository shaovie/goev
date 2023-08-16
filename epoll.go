package goev

import (
	"errors"
	"runtime"
	"sync"
	"syscall"
	"unsafe"
)

type evPoll struct {
	efd             int // epoll fd
	cachedTimestamp int64

	//ioReadWriter IOReadWriter
	evPollReadBuff  []byte
	evPollWriteBuff []byte

	evHandlerMap *evDataMap // Refer to https://zhuanlan.zhihu.com/p/640712548
	timer        *timer4Heap

	// async write
	asyncWrite *asyncWrite
}

func (ep *evPoll) open(evFdMaxSize int, timer *timer4Heap,
	evPollReadBuffSize, evPollWriteBuffSize int) error {
	efd, err := syscall.EpollCreate1(syscall.EPOLL_CLOEXEC)
	if err != nil {
		return errors.New("goev: epoll_create1 " + err.Error())
	}
	ep.efd = efd
	ep.timer = timer
	ep.evPollReadBuff = make([]byte, evPollReadBuffSize)
	ep.evPollWriteBuff = make([]byte, evPollWriteBuffSize)
	ep.evHandlerMap = newEvDataMap(evFdMaxSize)
	ep.asyncWrite, err = newAsyncWrite(ep)
	if err != nil {
		return err
	}

	// process max fds
	// show using `ulimit -Hn`
	// $GOROOT/src/os/rlimit.go Go had raise the limit to 'Max Hard Limit'
	return nil
}
func (ep *evPoll) updateCachedTime(t int64) {
	ep.cachedTimestamp = t
}
func (ep *evPoll) cachedTime() int64 {
	return ep.cachedTimestamp
}
func (ep *evPoll) loadEvData(fd int) *evData {
	return ep.evHandlerMap.load(fd)
}
func (ep *evPoll) add(fd int, events uint32, eh EvHandler) error {
	eh.setParams(fd, ep)

	ev := syscall.EpollEvent{Events: events}
	ed := ep.evHandlerMap.newOne(fd)
	ed.fd = fd
	ed.events = events
	ed.eh = eh
	ep.evHandlerMap.store(fd, ed) // 让evHandlerMap 来控制eh的生命周期, 不然会被gc回收的
	*(**evData)(unsafe.Pointer(&ev.Fd)) = ed

	if err := syscall.EpollCtl(ep.efd, syscall.EPOLL_CTL_ADD, fd, &ev); err != nil {
		// ENOSPC cat /proc/sys/fs/epoll/max_user_watches
		return errors.New("epoll_ctl add: " + err.Error())
	}
	return nil
}
func (ep *evPoll) remove(fd int) error {
	// The event argument is ignored and can be NULL (but see `man 2 epoll_ctl` BUGS)
	// kernel versions > 2.6.9
	ep.evHandlerMap.del(fd)
	if err := syscall.EpollCtl(ep.efd, syscall.EPOLL_CTL_DEL, fd, nil); err != nil {
		return errors.New("epoll_ctl del: " + err.Error())
	}
	return nil
}
func (ep *evPoll) append(fd int, events uint32) error {
	ed := ep.evHandlerMap.load(fd)
	if ed == nil {
		return errors.New("append: not found")
	}

	ev := syscall.EpollEvent{Events: events | ed.events}
	*(**evData)(unsafe.Pointer(&ev.Fd)) = ed

	if err := syscall.EpollCtl(ep.efd, syscall.EPOLL_CTL_MOD, fd, &ev); err != nil {
		return errors.New("epoll_ctl mod: " + err.Error())
	}
	ed.events |= events
	return nil
}
func (ep *evPoll) subtract(fd int, events uint32) error {
	ed := ep.evHandlerMap.load(fd)
	if ed == nil {
		return errors.New("subtract: not found")
	}

	ev := syscall.EpollEvent{Events: ed.events &^ events}
	*(**evData)(unsafe.Pointer(&ev.Fd)) = ed

	if err := syscall.EpollCtl(ep.efd, syscall.EPOLL_CTL_MOD, fd, &ev); err != nil {
		return errors.New("epoll_ctl mod: " + err.Error())
	}
	ed.events &= ^events
	return nil
}
func (ep *evPoll) scheduleTimer(eh EvHandler, delay, interval int64) (err error) {
	err = ep.timer.schedule(eh, delay, interval)
	return
}
func (ep *evPoll) cancelTimer(eh EvHandler) {
	ep.timer.cancel(eh)
}

// io handle
func (ep *evPoll) writeBuff() []byte {
	return ep.evPollWriteBuff
}
func (ep *evPoll) read(fd int) (bf []byte, n int, err error) {
	for {
		n, err = syscall.Read(fd, ep.evPollReadBuff)
		if n > 0 {
			bf = ep.evPollReadBuff[:n]
		} else if n < 0 && err == syscall.EINTR {
			continue
		}
		return
	}
}

func (ep *evPoll) push(awi asyncWriteItem) {
	ep.asyncWrite.push(awi)
}

// end of `io handle'

func (ep *evPoll) run(wg *sync.WaitGroup) error {
	if wg != nil {
		defer wg.Done()
	}

	var nfds, i, msec int
	var err error
	events := make([]syscall.EpollEvent, 256) // does not escape
	msec = -1
	for {
		nfds, err = syscall.EpollWait(ep.efd, events, msec)
		if nfds > 0 {
			msec = 0
			for i = 0; i < nfds; i++ {
				ev := &events[i]
				ed := *(**evData)(unsafe.Pointer(&ev.Fd))
				// EPOLLHUP refer to man 2 epoll_ctl
				if ev.Events&(syscall.EPOLLHUP|syscall.EPOLLERR) != 0 {
					ep.remove(ed.fd) // MUST before OnClose()
					ed.eh.OnClose()
					continue
				}
				if ev.Events&(syscall.EPOLLOUT) != 0 { // MUST before EPOLLIN (e.g. connect)
					if ed.eh.OnWrite() == false {
						ep.remove(ed.fd) // MUST before OnClose()
						ed.eh.OnClose()
						continue
					}
				}
				if ev.Events&(syscall.EPOLLIN) != 0 {
					if ed.eh.OnRead() == false {
						ep.remove(ed.fd) // MUST before OnClose()
						ed.eh.OnClose()
						continue
					}
				}
			} // end of `for i < nfds'
		} else if nfds == 0 || (nfds < 0 && err == syscall.EINTR) { // timeout
			msec = -1
			runtime.Gosched() // https://zhuanlan.zhihu.com/p/647958433
			continue
		} else if err != nil {
			return errors.New("syscall epoll_wait: " + err.Error())
		}
	}
}
