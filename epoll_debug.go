//go:build debug
// +build debug

package goev

import (
	"errors"
	"reflect"
	"runtime"
	"sync"
	"syscall"
	"unsafe"

	"golang.org/x/sys/unix"
)

var relEvHandlerUseMap bool = true // DEBUG

var (
	_zero uintptr
)

type epollEvent struct {
	Events uint32
	// Fd    [8]byte // 2023.6.29 测试发现用[8]byte的内存方式还是会出现崩溃的问题
	Fd  int32
	Pad int32
}

func errnoErr(e syscall.Errno) error {
	switch e {
	case 0:
		return nil
	case unix.EAGAIN:
		return syscall.EAGAIN
	case unix.EINVAL:
		return syscall.EINVAL
	case unix.ENOENT:
		return syscall.ENOENT
	}
	return e
}
func epollWait(epfd int, events []epollEvent, msec int) (n int, err error) {
	var _p0 unsafe.Pointer
	if len(events) > 0 {
		_p0 = unsafe.Pointer(&events[0])
	} else {
		_p0 = unsafe.Pointer(&_zero)
	}

	r0, _, e1 := unix.Syscall6(unix.SYS_EPOLL_WAIT, uintptr(epfd), uintptr(_p0), uintptr(len(events)), uintptr(msec), 0, 0)
	n = int(r0)
	if e1 != 0 {
		err = errnoErr(e1)
	}

	return
}
func epollCtl(epfd int, op int, fd int, event *epollEvent) (err error) {
	_, _, e1 := unix.RawSyscall6(unix.SYS_EPOLL_CTL, uintptr(epfd), uintptr(op), uintptr(fd), uintptr(unsafe.Pointer(event)), 0, 0)
	if e1 != 0 {
		err = errnoErr(e1)
	}

	return
}

// evData
type evData struct {
	fd        Fd
	evHandler EvHandler
}

func (ed *evData) reset(fd int, h EvHandler) {
	ed.fd.v = fd
	ed.evHandler = h
}

// evPoll
//
// Leader/Follower 模型, Leader负责epoll_wait, 当获取到I/O事件后, 转为Follower,
// 释放互斥锁并产生一个新的Leader, Follower负责处理I/O事件
// 最大程度实现并发处理I/O事件, 消除了线程间的数据切换, 和不必要的数据拷贝
type evPoll struct {
	efd int // epoll fd

	// 多个线程轮流执行epoll_wait, 获取到I/O事件, 马上通知其他
	pollThreadNum     int
	multiplePollerMtx sync.Mutex

	// TODO Put回收操作还没想好一个优雅的方式
	// evDataPool *sync.Pool

	evDataMap    sync.Map

	evPollSize int // epoll_wait一次轮询获取固定数量准备好的I/O事件, 此参数有利于多线程轮换
}

func (ep *evPoll) open(pollThreadNum, evPollSize int) error {
	var ev epollEvent
	if relEvHandlerUseMap && reflect.TypeOf(ev.Fd).Kind() != reflect.Int32 {
		panic("epollEvent struct Fd should be int32")
	}
	if pollThreadNum < 1 {
		return errors.New("EvPollThreadNum < 1")
	}
	if evPollSize < 1 {
		return errors.New("EvPollSize < 1")
	}
	efd, err := syscall.EpollCreate1(syscall.EPOLL_CLOEXEC)
	if err != nil {
		return errors.New("syscall epoll_create1: " + err.Error())
	}
	ep.evPollSize = evPollSize
	ep.efd = efd
	// ep.evDataPool = &sync.Pool{
	//     New: func() any {
	//         return new(evData)
	//     },
	// }
	ep.pollThreadNum = pollThreadNum
	// process max fds
	// show using `ulimit -Hn`
	// $GOROOT/src/os/rlimit.go Go had raise the limit to 'Max Hard Limit'
	return nil
}
func (ep *evPoll) add(fd, events int, h EvHandler) error {
	ed := &evData{} // TODO ep.evDataPool.Get().(*evData)
	ed.reset(fd, h)

	ev := epollEvent{
		Events: uint32(events),
	}
	if relEvHandlerUseMap {
        ev.Fd = int32(fd)
		ep.evDataMap.Store(fd, ed)
	} else {
		*(**evData)(unsafe.Pointer(&ev.Fd)) = ed
	}
	if err := epollCtl(ep.efd, syscall.EPOLL_CTL_ADD, fd, &ev); err != nil {
		return errors.New("epoll_ctl add: " + err.Error())
	}
	return nil
}
func (ep *evPoll) modify(fd, events int, h EvHandler) error {
	ed := &evData{} // TODO ep.evDataPool.Get().(*evData)
	ed.reset(fd, h)

	ev := epollEvent{
		Events: uint32(events),
	}
	if relEvHandlerUseMap {
        ev.Fd = int32(fd)
		ep.evDataMap.Store(fd, ed)
	} else {
		*(**evData)(unsafe.Pointer(&ev.Fd)) = ed
	}
	if err := epollCtl(ep.efd, syscall.EPOLL_CTL_MOD, fd, &ev); err != nil {
		if errors.Is(err, syscall.ENOENT) { // refer to `man 2 epoll_ctl`
			if err = epollCtl(ep.efd, syscall.EPOLL_CTL_ADD, fd, &ev); err != nil {
				return errors.New("epoll_ctl add: " + err.Error())
			}
			return nil
		}
		return errors.New("epoll_ctl mod: " + err.Error())
	}
	return nil
}
func (ep *evPoll) remove(fd int) error {
	// The event argument is ignored and can be NULL (but see `man 2 epoll_ctl` BUGS)
	// kernel versions > 2.6.9
	if relEvHandlerUseMap {
		ep.evDataMap.Delete(fd)
	}
	if err := epollCtl(ep.efd, syscall.EPOLL_CTL_DEL, fd, nil); err != nil {
		return errors.New("epoll_ctl del: " + err.Error())
	}
	return nil
}
func (ep *evPoll) run() (err error) {
	if ep.pollThreadNum == 1 {
		return ep.poll(false, nil)
	}
	var wg sync.WaitGroup
	for i := 0; i < ep.pollThreadNum; i++ {
		wg.Add(1)
		go func() {
			err = ep.poll(true, &wg)
		}()
	}
	wg.Wait()
	return err
}
func (ep *evPoll) poll(multiplePoller bool, wg *sync.WaitGroup) error {
	if wg != nil {
		defer wg.Done()
	}

	// Refer to go doc runtime.LockOSThread
	// LockOSThread will bind the current goroutine to the current OS thread T,
	// preventing other goroutines from being scheduled onto this thread T
	runtime.LockOSThread()

	var nfds int
	var err error
	// $GOROOT/src/syscall/ztypes_linux_amd64.go
	events := make([]epollEvent, ep.evPollSize)
	for {
		if multiplePoller == true {
			ep.multiplePollerMtx.Lock()
			nfds, err = epollWait(ep.efd, events, -1)
			ep.multiplePollerMtx.Unlock()
		} else {
			nfds, err = epollWait(ep.efd, events, -1)
		}
		if nfds > 0 {
			for i := 0; i < nfds; i++ {
				ev := &events[i]
				var ed *evData
				if relEvHandlerUseMap {
                    if ted, ok := ep.evDataMap.Load(int(ev.Fd)); ok {
                        ed = ted.(*evData)
                    } else {
						panic("evDataMap not found") // DEBUG
					}
				} else {
					ed = *(**evData)(unsafe.Pointer(&ev.Fd))
				}
				// EPOLLHUP refer to man 2 epoll_ctl
				if ev.Events&(syscall.EPOLLHUP|syscall.EPOLLERR) != 0 {
					ep.remove(ed.fd.v)
					ed.evHandler.OnClose(&(ed.fd))
					continue
				}
				if ev.Events&(syscall.EPOLLOUT) != 0 {
					if ed.evHandler.OnWrite(&(ed.fd)) == false {
						ep.remove(ed.fd.v)
						ed.evHandler.OnClose(&(ed.fd))
						continue
					}
				}
				if ev.Events&(syscall.EPOLLIN) != 0 {
					if ed.evHandler.OnRead(&(ed.fd)) == false {
						ep.remove(ed.fd.v)
						ed.evHandler.OnClose(&(ed.fd))
						continue
					}
				}
			} // end of `for i < nfds'
		} else if nfds == 0 {
			continue
		} else if err != nil && !errors.Is(err, syscall.EINTR) { // nfds < 0
			return errors.New("syscall epoll_wait: " + err.Error())
		}
	}
	return nil
}
