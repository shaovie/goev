package goev

type Options struct {
	noCopy

	// acceptor options
	reuseAddr     bool // SO_REUSEADDR
	listenBacklog int  //

	// connector options

	// acceptor and connector options
	sockRcvBufSize int // ignore equal 0

	// reactor options
	evPollNum            int //
	evReadyNum           int //
	evDataArrSize        int
	evPollLockOSThread   bool
	evPollSharedBuffSize int

	// timer
	noTimer           bool
	timerHeapInitSize int //
}

type Option func(*Options)

var evOptions *Options

func setOptions(optL ...Option) {
	if evOptions == nil {
		//= defaut options
		evOptions = &Options{
			reuseAddr:            true,
			evPollNum:            1,
			evReadyNum:           512,
			evDataArrSize:        8192,
			listenBacklog:        512, // go default 128
			noTimer:              false,
			timerHeapInitSize:    1024,
			evPollLockOSThread:   false,
			evPollSharedBuffSize: 64 * 1024,
		}
	}

	for _, opt := range optL {
		opt(evOptions)
	}
}

// For SO_REUSEADDR
func ReuseAddr(v bool) Option {
	return func(o *Options) {
		o.reuseAddr = v
	}
}

// Listen backlog, For syscall.listen(fd, backlog), also affect `for i < backlog/2 { syscall.accept() }`
func ListenBacklog(v int) Option {
	return func(o *Options) {
		o.listenBacklog = v
	}
}

// For SO_RCVBUF, for new sockfd in acceptor/connector
func SockRcvBufSize(n int) Option {
	return func(o *Options) {
		o.sockRcvBufSize = n
	}
}

// ArrayMapUnion数据结构中array的容易, 性能不会线性增长, 主要根据自己的服务中fd并发数量(fd=0~n的范围)来定
func EvDataArrSize(n int) Option {
	return func(o *Options) {
		if n > 0 {
			o.evDataArrSize = n
		}
	}
}

// 是否绑定固定线程 请参考go doc runtime.LockOSThread (经过实测, 会降低约2%的性能)
func EvPollLockOSThread(v bool) Option {
	return func(o *Options) {
		o.evPollLockOSThread = v
	}
}

// evpoll数量, 每个evpoll就像是在独立的线程中运行, 建议用CPUx2-1(注意留出其他goroutine的cpu)
// 网络程序I/O密集, cpu切换会比较频繁, 所以1个cpu绑定2个evpoll最好
func EvPollNum(n int) Option {
	return func(o *Options) {
		if n > 0 {
			o.evPollNum = n
		}
	}
}

// evpoll一次轮询获取数量n的Ready I/O事件, 有利于提高批量处理能力, 太大容易影响新事件的处理
func EvReadyNum(n int) Option {
	return func(o *Options) {
		if n > 0 {
			o.evReadyNum = n
		}
	}
}

// 单个evpoll内的全局共享内存, 对cpu cache 友好, 读取socket缓存区的数据时非常高效
// 另: 如果是Epoll-ET模式, 就需要有足够大的内存来一次性读完缓冲区的数据
func EvPollSharedBuffSize(n int) Option {
	return func(o *Options) {
		if n > 0 {
			o.evPollSharedBuffSize = n
		}
	}
}

// timer
func TimerHeapInitSize(n int) Option {
	return func(o *Options) {
		if n > 0 {
			o.timerHeapInitSize = n
		}
	}
}

// 不使用Timer后, OnOpen/OnRead/OnWrite中的时间值就会是0
func NoTimer(v bool) Option {
	return func(o *Options) {
		o.noTimer = v
	}
}
