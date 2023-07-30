package goev

import (
	"errors"
	"sync"
	"syscall"
	"time"
	"unsafe"
)

type evData struct {
	fd int
	eh EvHandler
}

// evPoll
type evPoll struct {
	efd int // epoll fd

	evReadyNum   int // epoll_wait一次轮询获取固定数量准备好的I/O事件, 此参数有利于线程处理的敏捷性
	ioReadWriter IOReadWriter

	evHandlerMap *ArrayMapUnion[evData] // Refer to https://zhuanlan.zhihu.com/p/640712548
	timer        timer
	evPollWakeup *notify
}

func (ep *evPoll) open(evReadyNum, evDataArrSize int,
	timer timer, ioReadWriter IOReadWriter) error {
	if evReadyNum < 1 {
		return errors.New("EvReadyNum < 1")
	}
	efd, err := syscall.EpollCreate1(syscall.EPOLL_CLOEXEC)
	if err != nil {
		return errors.New("syscall epoll_create1: " + err.Error())
	}
	ep.efd = efd
	ep.timer = timer
	ep.evReadyNum = evReadyNum
	ep.ioReadWriter = ioReadWriter
	ep.evHandlerMap = NewArrayMapUnion[evData](evDataArrSize)

	// Must be placed last
	ep.evPollWakeup, err = newNotify(ep)
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

	ev := syscall.EpollEvent{Events: events}
	ed := &evData{fd: fd, eh: eh}
	ep.evHandlerMap.Store(fd, ed) // 让evHandlerMap 来控制eh的生命周期, 不然会被gc回收的
	*(**evData)(unsafe.Pointer(&ev.Fd)) = ed

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
	if ep.timer == nil {
		return errors.New("not create timer")
	}
	eh.setEvPoll(ep)
	err = ep.timer.schedule(eh, delay, interval)
	ep.evPollWakeup.Notify()
	return
}
func (ep *evPoll) cancelTimer(eh EvHandler) {
	if ep.timer != nil {
		ep.timer.cancel(eh)
	}
}
func (ep *evPoll) run(wg *sync.WaitGroup) error {
	if wg != nil {
		defer wg.Done()
	}

	var nfds, i, msec int
	var err error
	var now int64
	msec = -1
	events := make([]syscall.EpollEvent, ep.evReadyNum)
	for {
		nfds, err = syscall.EpollWait(ep.efd, events, msec)
		if ep.timer != nil {
			now = time.Now().UnixMilli()
			msec = int(ep.timer.handleExpired(now))
		}
		if nfds > 0 {
			for i = 0; i < nfds; i++ {
				ev := &events[i]
				ed := *(**evData)(unsafe.Pointer(&ev.Fd))
				// EPOLLHUP refer to man 2 epoll_ctl
				if ev.Events&(syscall.EPOLLHUP|syscall.EPOLLERR) != 0 {
					ep.remove(ed.fd) // MUST before OnClose()
					ed.eh.OnClose(ed.fd)
					continue
				}
				if ev.Events&(syscall.EPOLLOUT) != 0 { // MUST before EPOLLIN (e.g. connect)
					if ed.eh.OnWrite(ed.fd, ep.ioReadWriter, now) == false {
						ep.remove(ed.fd) // MUST before OnClose()
						ed.eh.OnClose(ed.fd)
						continue
					}
				}
				if ev.Events&(syscall.EPOLLIN) != 0 {
					if ed.eh.OnRead(ed.fd, ep.ioReadWriter, now) == false {
						ep.remove(ed.fd) // MUST before OnClose()
						ed.eh.OnClose(ed.fd)
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
}
