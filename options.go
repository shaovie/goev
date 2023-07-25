package goev

// Options provides all optional parameters within the framework
type Options struct {
	noCopy

	// acceptor options
	reuseAddr     bool // SO_REUSEADDR
	reusePort     bool // SO_REUSEPORT
	listenBacklog int  //

	// connector options

	// acceptor and connector options
	sockRcvBufSize int // ignore equal 0

	// reactor options
	evPollNum          int //
	evReadyNum         int //
	evDataArrSize      int
	evPollLockOSThread bool
	ioReadWriter       IOReadWriter

	// timer
	noTimer           bool
	timerHeapInitSize int //
}

// Option function
type Option func(*Options)

func setOptions(optL ...Option) *Options {
	//= defaut options
	opts := &Options{
		reuseAddr:          true,
		reusePort:          false,
		evPollNum:          1,
		evReadyNum:         512,
		evDataArrSize:      8192,
		listenBacklog:      512, // go default 128
		noTimer:            false,
		timerHeapInitSize:  1024,
		evPollLockOSThread: false,
		ioReadWriter:       NewIOReadWriter(64*1024, 1024*1024),
	}

	for _, opt := range optL {
		opt(opts)
	}
	return opts
}

// ReuseAddr for SO_REUSEADDR
func ReuseAddr(v bool) Option {
	return func(o *Options) {
		o.reuseAddr = v
	}
}

// ReusePort for SO_REUSEPORT
//
// Requires kernel >= 3.9
// Please make sure you have a good understanding of SO_REUSEPORT.(man 7 socket)
// For example code, please refer to example/reuseport.go
func ReusePort(v bool) Option {
	return func(o *Options) {
		o.reusePort = v
	}
}

// ListenBacklog For syscall.listen(fd, backlog), also affect `for i < backlog/2 { syscall.accept() }`
func ListenBacklog(v int) Option {
	return func(o *Options) {
		o.listenBacklog = v
	}
}

// SockRcvBufSize for SO_RCVBUF, for new sockfd in acceptor/connector
func SockRcvBufSize(n int) Option {
	return func(o *Options) {
		o.sockRcvBufSize = n
	}
}

// EvDataArrSize for ArrayMapUnion数据结构中array的容易, 性能不会线性增长,
// 主要根据自己的服务中fd并发数量(fd=0~n的范围)来定
func EvDataArrSize(n int) Option {
	return func(o *Options) {
		if n > 0 {
			o.evDataArrSize = n
		}
	}
}

// EvPollLockOSThread Whether binds to a fixed thread.
// please refer to the go doc runtime.LockOSThread (After testing, it is found to
// decrease performance by approximately 2%)
//
// EvPollLockOSThread 是否绑定固定线程 请参考go doc runtime.LockOSThread (经过实测, 会降低约2%的性能)
func EvPollLockOSThread(v bool) Option {
	return func(o *Options) {
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
	return func(o *Options) {
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
func EvReadyNum(n int) Option {
	return func(o *Options) {
		if n > 0 {
			o.evReadyNum = n
		}
	}
}

// EvPollSharedBuffSize is the global shared memory within a single evpoll,
// which is friendly to CPU cache and highly efficient when reading data from socket buffers.
// Additionally, if it is Epoll-ET mode, there needs to be a sufficiently large amount of
// memory to read all the data from the buffer at once
//
// 单个evpoll内的全局共享内存, 对cpu cache 友好, 读取socket缓存区的数据时非常高效
// 另: 如果是Epoll-ET模式, 就需要有足够大的内存来一次性读完缓冲区的数据
func EvPollSharedBuffSize(n int) Option {
	panic("deprecated")
}

// SetIOReadWriter setting IOReadWriter
//
// default IOReadWriter buf size is 64KB,
// Reading data can increase the buffer capacity, but writing data does not impose any restrictions
// on the buffer capacity
func SetIOReadWriter(rw IOReadWriter) Option {
	return func(o *Options) {
		o.ioReadWriter = rw
	}
}

// TimerHeapInitSize is the initial array size of the heap structure used to implement timers
func TimerHeapInitSize(n int) Option {
	return func(o *Options) {
		if n > 0 {
			o.timerHeapInitSize = n
		}
	}
}

// NoTimer can be used to specify that no timer object should be created internally within the Reactor.
// In addition, the time values in OnOpen, OnRead, and OnWrite will also be set to 0.
// This can slightly improve performance for applications that do not require a timer.
// However, if you need a Connector, you should never set Notimer=true.
//
// NoTimer 可以指定Reactor内部不创建定时器对象，并且在OnOpen OnRead OnWrite中的时间值也会是0，
// 这样对于不需要定时器的应用，可以提高一点点性能（如果你需要Connector那么绝对不能设置Notimer=true)
func NoTimer(v bool) Option {
	return func(o *Options) {
		o.noTimer = v
	}
}
