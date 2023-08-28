package goev

import (
	"errors"
	"runtime"
	"sync"
	"syscall"
	"unsafe"
)

type evPoll struct {
	id  int //
	efd int // epoll fd

	evPollReadBuff  []byte
	evPollWriteBuff []byte

	evHandlerMap *evDataMap // Refer to https://zhuanlan.zhihu.com/p/640712548
	timer        *timer4Heap

	asyncWrite       *asyncWrite
	pollSyncOpterate *pollSyncOpt
	pCache           map[int]any
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
	ep.pCache = make(map[int]any, 16)
	ep.evHandlerMap = newEvDataMap(evFdMaxSize)
	if ep.asyncWrite, err = newAsyncWrite(ep); err != nil {
		return err
	}
	if ep.pollSyncOpterate, err = newPollSyncOpt(ep); err != nil {
		return err
	}

	// process max fds
	// show using `ulimit -Hn`
	// $GOROOT/src/os/rlimit.go Go had raise the limit to 'Max Hard Limit'
	return nil
}

// first register
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
func (ep *evPoll) remove(fd int, events uint32) error {
	if events == EvAll {
		// The event argument is ignored and can be NULL (but see `man 2 epoll_ctl` BUGS)
		// kernel versions > 2.6.9
		ep.evHandlerMap.del(fd)
		if err := syscall.EpollCtl(ep.efd, syscall.EPOLL_CTL_DEL, fd, nil); err != nil {
			return errors.New("epoll_ctl del: " + err.Error())
		}
		return nil
	}
	ed := ep.evHandlerMap.load(fd)
	if ed == nil {
		return errors.New("remove: not found")
	}

	if ed.events&^events == 0 {
		ep.evHandlerMap.del(fd)
		if err := syscall.EpollCtl(ep.efd, syscall.EPOLL_CTL_DEL, fd, nil); err != nil {
			return errors.New("epoll_ctl del: " + err.Error())
		}
		return nil
	}

	// Always save first, recover if failed  (this method is for multi-threading scenarios)."
	ed.events &= ^events

	ev := syscall.EpollEvent{Events: ed.events}
	*(**evData)(unsafe.Pointer(&ev.Fd)) = ed
	if err := syscall.EpollCtl(ep.efd, syscall.EPOLL_CTL_MOD, fd, &ev); err != nil {
		ed.events |= events;
		return errors.New("epoll_ctl mod: " + err.Error())
	}
	return nil
}
func (ep *evPoll) append(fd int, events uint32) error {
	ed := ep.evHandlerMap.load(fd)
	if ed == nil {
		return errors.New("append: not found")
	}

	// Always save first, recover if failed  (this method is for multi-threading scenarios)."
	ed.events |= events

	ev := syscall.EpollEvent{Events: ed.events}
	*(**evData)(unsafe.Pointer(&ev.Fd)) = ed
	if err := syscall.EpollCtl(ep.efd, syscall.EPOLL_CTL_MOD, fd, &ev); err != nil {
		ed.events &= ^events
		return errors.New("epoll_ctl mod: " + err.Error())
	}
	return nil
}
func (ep *evPoll) run(wg *sync.WaitGroup) error {
	if wg != nil {
		defer wg.Done()
	}

	var nfds, i, msec int
	var err error
	events := make([]syscall.EpollEvent, 128) // does not escape (该值不是越大越好)
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
					eh := ed.eh
					ep.remove(ed.fd, EvAll) // MUST before OnClose()
					eh.OnClose()
					continue
				}
				if ev.Events&(syscall.EPOLLOUT) != 0 { // MUST before EPOLLIN (e.g. connect)
					if ed.eh.OnWrite() == false {
						eh := ed.eh
						ep.remove(ed.fd, EvAll) // MUST before OnClose()
						eh.OnClose()
						continue
					}
				}
				if ev.Events&(syscall.EPOLLIN) != 0 {
					if ed.eh.OnRead() == false {
						eh := ed.eh
						ep.remove(ed.fd, EvAll) // MUST before OnClose()
						eh.OnClose()
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

func (ep *evPoll) scheduleTimer(eh EvHandler, delay, interval int64) error {
	return ep.timer.schedule(eh, delay, interval)
}
func (ep *evPoll) cancelTimer(eh EvHandler) {
	ep.timer.cancel(eh)
}

// poll sync opt
func (ep *evPoll) initPollSyncOpt(typ int, val any) {
	ep.pollSyncOpterate.init(typ, val)
}
func (ep *evPoll) pollSyncOpt(typ int, val any) {
	ep.pollSyncOpterate.push(typ, val)
}
func (ep *evPoll) pCacheSet(id int, val any) {
	ep.pCache[id] = val
}
func (ep *evPoll) pCacheGet(id int) (any, bool) {
	if v, ok := ep.pCache[id]; ok {
		return v, true
	}
	return nil, false
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
