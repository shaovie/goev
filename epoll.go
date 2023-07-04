package goev

import (
	"errors"
	"runtime"
	"sync"
	"syscall"
	"time"
)

// evData
type evData struct {
	noCopy
	fd        Fd
	evHandler EvHandler
}

func (ed *evData) reset(fd int, h EvHandler) {
	ed.fd.reset(fd)
	ed.evHandler = h
}

// evPoll
//
// Leader/Follower 模型, Leader负责epoll_wait, 当获取到I/O事件后, 转为Follower,
// 释放互斥锁并产生一个新的Leader, Follower负责处理I/O事件
// 最大程度实现并发处理I/O事件, 消除了线程间的数据切换, 和不必要的数据拷贝
type evPoll struct {
	efd int // epoll fd

	evReadyNum int // epoll_wait一次轮询获取固定数量准备好的I/O事件, 此参数有利于多线程轮换

	evDataPool   *sync.Pool
	evDataMap    *ArrayMapUnion[evData]
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
	ep.evDataPool = &sync.Pool{
		New: func() any {
			return new(evData)
		},
	}
	ep.evDataMap = NewArrayMapUnion[evData](evDataArrSize)
	ep.timer = timer

	for i := 0; i < 16; i++ { // warm up
		ep.evDataPool.Put(new(evData))
	}

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
	eh.init(ep, fd)
	ed := ep.evDataPool.Get().(*evData)
	ed.reset(fd, eh)

	ev := syscall.EpollEvent{
		Events: events,
		Fd:     int32(fd),
	}
	ep.evDataMap.Store(fd, ed)

	if err := syscall.EpollCtl(ep.efd, syscall.EPOLL_CTL_ADD, fd, &ev); err != nil {
		return errors.New("epoll_ctl add: " + err.Error())
	}
	return nil
}
func (ep *evPoll) remove(fd int) error {
	// The event argument is ignored and can be NULL (but see `man 2 epoll_ctl` BUGS)
	// kernel versions > 2.6.9
	ep.evDataMap.Delete(fd)
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

	var nfds, i int
	var err error
	var now int64
	msec := -1
	events := make([]syscall.EpollEvent, ep.evReadyNum) // NOT make(x, len, cap)
	for {
		nfds, err = syscall.EpollWait(ep.efd, events, msec)
		now = time.Now().UnixMilli()
		msec = int(ep.timer.handleExpired(now))
		if nfds > 0 {
			for i = 0; i < nfds; i++ {
				ev := &events[i]
				ed := ep.evDataMap.Load(int(ev.Fd))
				if ed == nil {
					continue // TODO add evOptions.debug? panic("evDataMap not found")
				}
				// EPOLLHUP refer to man 2 epoll_ctl
				if ev.Events&(syscall.EPOLLHUP|syscall.EPOLLERR) != 0 {
					ep.remove(ed.fd.v) // MUST before OnClose()
					ed.evHandler.OnClose(&(ed.fd))
					ep.evDataPool.Put(ed)
					continue
				}
				if ev.Events&(syscall.EPOLLOUT) != 0 { // MUST before EPOLLIN (e.g. connect)
					if ed.evHandler.OnWrite(&(ed.fd), now) == false {
						ep.remove(ed.fd.v) // MUST before OnClose()
						ed.evHandler.OnClose(&(ed.fd))
						ep.evDataPool.Put(ed)
						continue
					}
				}
				if ev.Events&(syscall.EPOLLIN) != 0 {
					if ed.evHandler.OnRead(&(ed.fd), now) == false {
						ep.remove(ed.fd.v) // MUST before OnClose()
						ed.evHandler.OnClose(&(ed.fd))
						ep.evDataPool.Put(ed)
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
