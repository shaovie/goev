package goev

import (
)

type Options struct {
    noCopy

	// acceptor options
	reuseAddr     bool // SO_REUSEADDR
	listenBacklog int  //

	// connector options

	// acceptor and connector options
	recvBuffSize  int  // ignore equal 0

	// reactor options
	evPollNum      int //
	evReadyNum     int //
    evDataArrSize  int

    // timer
    timerHeapInitSize int //
}

type Option func(*Options)

var evOptions *Options

func setOptions(optL ...Option) {
	if evOptions == nil {
		//= defaut options
		evOptions = &Options{
			reuseAddr:     true,
			evPollNum:    1,
			evReadyNum:   512,
			evDataArrSize: 8192,
			listenBacklog: 1024, // go default 128
            timerHeapInitSize: 1024,
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
func RecvBuffSize(n int) Option {
	return func(o *Options) {
		o.recvBuffSize = n
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

// evpoll数量, 每个evpoll是个独立线程在运行, 建议跟cpu个数绑定(注意留出其他goroutine的cpu)
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

// timer
func TimerHeapInitSize(n int) Option {
	return func(o *Options) {
        if n > 0 {
            o.timerHeapInitSize = n
        }
	}
}
