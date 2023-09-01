package goev

import (
	"sync"
)

// global option
var (
	ioBuffUsePool        bool = true
	setIOBuffUsePoolOnce sync.Once
)

// SetIOBuffUsePool switch
func SetIOBuffUsePool(v bool) {
	setIOBuffUsePoolOnce.Do(func() {
		ioBuffUsePool = v
	})
}

// options provides all optional parameters within the framework
type options struct {
	// acceptor options
	reuseAddr     bool // SO_REUSEADDR
	reusePort     bool // SO_REUSEPORT
	listenBacklog int  //

	// connector options

	// acceptor and connector options
	sockRcvBufSize int // ignore equal 0

	// reactor options
	evPollLockOSThread  bool
	evPollNum           int //
	evFdMaxSize         int
	evPollReadBuffSize  int
	evPollWriteBuffSize int
	//evPollCacheTimePeriod int

	// timer
	timerHeapInitSize int //
}

// Option function
type Option func(*options)

func setOptions(optL ...Option) options {
	//= defaut options
	opts := options{
		reuseAddr:         true,
		reusePort:         false,
		evPollNum:         1,
		evFdMaxSize:       8192,
		listenBacklog:     512, // go default 128
		timerHeapInitSize: 1024,
		//evPollCacheTimePeriod: 250,
		evPollLockOSThread:  false,
		evPollReadBuffSize:  8192,
		evPollWriteBuffSize: 16 * 1024,
	}

	for _, opt := range optL {
		opt(&opts)
	}
	return opts
}

// ReuseAddr for SO_REUSEADDR
func ReuseAddr(v bool) Option {
	return func(o *options) {
		o.reuseAddr = v
	}
}

// ReusePort for SO_REUSEPORT
//
// Requires kernel >= 3.9
// Please make sure you have a good understanding of SO_REUSEPORT.(man 7 socket)
// For example code, please refer to example/reuseport.go
func ReusePort(v bool) Option {
	return func(o *options) {
		o.reusePort = v
	}
}

// ListenBacklog For syscall.listen(fd, backlog), also affect `for i < backlog/2 { syscall.accept() }`
func ListenBacklog(v int) Option {
	return func(o *options) {
		o.listenBacklog = v
	}
}

// SockRcvBufSize for SO_RCVBUF, for new sockfd in acceptor/connector
func SockRcvBufSize(n int) Option {
	return func(o *options) {
		o.sockRcvBufSize = n
	}
}

// EvFdMaxSize for ArrayMapUnion数据结构中array的容量, 性能不会线性增长,
// 主要根据自己的服务中fd并发数量(fd=0~n的范围)来定
// fd数量超过此值并不会拒绝服务, 只是存储结构切换到map
func EvFdMaxSize(n int) Option {
	return func(o *options) {
		if n > 0 {
			o.evFdMaxSize = n
		}
	}
}

// EvPollLockOSThread Whether binds to a fixed thread.
// please refer to the go doc runtime.LockOSThread (After testing, it is found to
// decrease performance by approximately 2%)
//
// EvPollLockOSThread 是否绑定固定线程 请参考go doc runtime.LockOSThread (经过实测, 会降低约2%的性能)
func EvPollLockOSThread(v bool) Option {
	return func(o *options) {
		o.evPollLockOSThread = v
	}
}

// EvPollNum is the number of evPoll instances, with each evPoll instance running in an independent thread.
// It is recommended to use CPUx2-1 (taking into account other goroutines' CPU usage) for network
// programs that are I/O intensive and involve frequent CPU switching.
// Therefore, it is best to bind two epoll instances to one CPU.
//
// EvPollNum evpoll数量, 每个evpoll就像是在独立的线程中运行, 建议用CPUx2-1(注意留出其他goroutine的cpu)
// 网络程序I/O密集, cpu切换会比较频繁, 所以1个cpu绑定2个evpoll最好
func EvPollNum(n int) Option {
	return func(o *options) {
		if n > 0 {
			o.evPollNum = n
		}
	}
}

// EvReadyNum evPolling for a quantity of n Ready I/O events at once is beneficial for improving
// batch processing capability. However, if the quantity is too large,
// it can easily impact the processing of new events.
//
// EvReadyNum evpoll一次轮询获取数量n的Ready I/O事件, 有利于提高批量处理能力, 太大容易影响新事件的处理
//func EvReadyNum(n int) Option {
//	return func(o *options) {
//		if n > 0 {
//			o.evReadyNum = n
//		}
//	}
//}

// EvPollReadBuffSize is the global shared memory within a single evpoll,
// which is friendly to CPU cache and highly efficient when reading data from socket buffers.
// Additionally, if it is Epoll-ET mode, there needs to be a sufficiently large amount of
// memory to read all the data from the buffer at once
//
// 单个evpoll内的全局共享内存, 对cpu cache 友好, 读取socket缓存区的数据时非常高效
// 另: 如果是Epoll-ET模式, 就需要有足够大的内存来一次性读完缓冲区的数据
func EvPollReadBuffSize(n int) Option {
	return func(o *options) {
		if n > 0 {
			o.evPollReadBuffSize = n
		}
	}
}

// EvPollWriteBuffSize is the global shared memory within a single evpoll,
func EvPollWriteBuffSize(n int) Option {
	return func(o *options) {
		if n > 0 {
			o.evPollWriteBuffSize = n
		}
	}
}

// EvPollCacheTimePeriod refer to whether caching timestamps within the 'evpoll' range is
// enabled (updated every `period' milliseconds)
//
// Turn off cache time if `period' equal zero
//func EvPollCacheTimePeriod(period int) Option {
//	return func(o *options) {
//		o.evPollCacheTimePeriod = period
//	}
//}

// TimerHeapInitSize is the initial array size of the heap structure used to implement timers
func TimerHeapInitSize(n int) Option {
	return func(o *options) {
		if n > 0 {
			o.timerHeapInitSize = n
		}
	}
}
