package goev

import (
	"runtime"
)

type Options struct {
	// acceptor options
	reuseAddr     bool // SO_REUSEADDR
	listenBacklog int  //

	// connector options

	// acceptor and connector options
	recvBuffSize  int  // ignore equal 0

	// reactor options
	evPollSize      int //
	evPollThreadNum int //
    evDataArrSize  int
}

type Option func(*Options)

var evOptions *Options

func setOptions(optL ...Option) {
	if evOptions == nil {
		//= defaut options
		evOptions = &Options{
			reuseAddr:     true,
			evPollSize:    512,
			evDataArrSize: 8192,
			listenBacklog: 1024, // go default 128
		}
        cpuN := runtime.NumCPU()
        evOptions.evPollThreadNum = 1
        if cpuN > 15 {
			evOptions.evPollThreadNum = cpuN - 4
        } else if cpuN > 3 {
			evOptions.evPollThreadNum = cpuN - 2
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

// evpoll一次轮询获取数量n的Ready I/O事件, 此参数有利于多线程并发处理I/O事件
func EvPollSize(n int) Option {
	return func(o *Options) {
        if n > 0 {
            o.evPollSize = n
        }
	}
}

// 多个线程轮流执行event poll. poll线程获取到I/O事件后, 马上唤醒空闲的线程执行event loop, 依次循环
func EvPollThreadNum(n int) Option {
	return func(o *Options) {
        if n > 0 {
            o.evPollThreadNum = n
        }
	}
}
