# Event-driven network framework in Go
[中文文档](https://zhuanlan.zhihu.com/p/648641683)

Goev provides a high-performance, lightweight, non-blocking, I/O event-driven networking framework for the Go language. It draws inspiration from the design patterns of [ACE](http://www.dre.vanderbilt.edu/~schmidt/ACE-overview.html) and provides an elegant and concise solution for TCP network programming projects. With goev, you can seamlessly integrate your projects without worrying about the coroutine pressure introduced by the standard library (go net).

Moreover, goev excels in terms of performance. In the TechEmpower benchmark tests, it has achieved first place among similar frameworks in the same environment (goev has been submitted to TechEmpower and is awaiting the next round of public evaluation).

![](goev.png)
## Features

* I/O event-driven architecture
* Lightweight and easy-to-use
* Supporting asynchronous sending allows higher-level applications to perform synchronous I/O operations while asynchronously handling business processing
* Support multi-threaded polling
* Perfect support for REUSEPORT multi-threading mode
* Lock-free operations in a polling stack
* Built-in quad-heap timer, suitable for performance-demanding scenarios with a large number of timers
* Provide multiple optimization options
* Build-in connection pool
* Few APIs and low learning costs

## Installation

```bash
go get -u github.com/shaovie/goev
```

## Getting Started

See the [中文指南](DOCUMENT_CN.md) for the Chinese documentation.

### Simple Service Example

```go
package main

import (
    "github.com/shaovie/goev"
)

var forNewFdReactor *goev.Reactor

type Http struct {
	goev.Event
}

func (h *Http) OnOpen(fd int) bool {
	if err := forNewFdReactor.AddEvHandler(h, fd, goev.EvIn); err != nil {
		return false
	}
	return true
}
func (h *Http) OnRead() bool {
	_, n, _ := h.Read()
	if n == 0 { // Abnormal connection
		return false
	}
    return true
}
func (h *Http) OnClose() {
    if h.Fd() != -1 {
        netfd.Close(h.Fd())
        h.Destroy(h)
    }
}

func main() {
	runtime.GOMAXPROCS(runtime.NumCPU()*2 - 1)
	forAcceptReactor, err := goev.NewReactor(
		goev.EvPollNum(1),
	)
	if err != nil {
		panic(err.Error())
	}
	forNewFdReactor, err := goev.NewReactor(
		goev.EvPollNum(runtime.NumCPU()*2-1),
	)
	if err != nil {
		panic(err.Error())
	}
	_, err = goev.NewAcceptor(forAcceptReactor, func() goev.EvHandler { return new(Http) },
		":8080",
		goev.ListenBacklog(256),
		goev.SockRcvBufSize(16*1024),
	)
	if err != nil {
		panic(err.Error())
	}

	go func() {
		if err = forAcceptReactor.Run(); err != nil {
			panic(err.Error())
		}
	}()
    
	if err = forNewFdReactor.Run(); err != nil {
		panic(err.Error())
	}
}

```

### REUSEPORT Service Example

```go
package main

...

func main() {
	runtime.GOMAXPROCS(runtime.NumCPU()*2 - 1)
    evPollNum := runtime.NumCPU()*2-1
	forNewFdReactor, err := goev.NewReactor(
		goev.EvPollNum(evPollNum),
	)
	if err != nil {
		panic(err.Error())
	}
    for i := 0; i < evPollNum; i++ {
        _, err = goev.NewAcceptor(forNewFdReactor, func() goev.EvHandler { return new(Http) },
            ":8080",
            goev.ListenBacklog(256),
            goev.SockRcvBufSize(16*1024),
            goev.ReusePort(true),
        )
        if err != nil {
            panic(err.Error())
        }
    }
	if err = forNewFdReactor.Run(); err != nil {
		panic(err.Error())
	}
}

```
> Note: The reactor will bind different acceptors (listener fd) to different epoll instances to achieve multithreaded concurrent listening on the same IP:PORT

## Benchmarks

We're comparing gnet, which is ranked first on TechEmpower, using the test code from [gnet (GoLang) Benchmarking Test](https://github.com/TechEmpower/FrameworkBenchmarks/tree/master/frameworks/Go/gnet)

> Test environment GCP cloud VM, 2 cores, 4GB RAM

The bench results of gnet.
```text
wrk -c 2 -t 2 -d10s http://127.0.0.1:8080/xxx
Running 10s test @ http://127.0.0.1:8080/xxx
  2 threads and 2 connections
  Thread Stats   Avg      Stdev     Max   +/- Stdev
    Latency    47.79us  170.86us   8.95ms   99.59%
    Req/Sec    22.92k     1.00k   24.39k    78.11%
  458456 requests in 10.10s, 56.40MB read
Requests/sec:  45395.26
Transfer/sec:      5.58MB
```

The bench results of goev. [test code](https://github.com/shaovie/goev/blob/main/example/techempower.go)
```text
wrk -c 2 -t 2 -d10s http://127.0.0.1:8080/xxx
Running 10s test @ http://127.0.0.1:8080/xxx
  2 threads and 2 connections
  Thread Stats   Avg      Stdev     Max   +/- Stdev
    Latency    42.69us   92.32us   6.33ms   99.57%
    Req/Sec    23.49k     1.77k   26.37k    54.95%
  471993 requests in 10.10s, 68.87MB read
Requests/sec:  46733.75
Transfer/sec:      6.82MB
```
> Note: This is the most basic and simplest test, for reference only

## Why high-performance

* Connection bind threads/coroutines, no need for mutex locks within the 'polling stack' loop, provide global shared memory within the 'polling stack' for easy data reading, saving memory, and avoiding frequent memory allocation (also unnecessary for mutex locks)
* All operations directly use syscall, avoiding the use of encapsulation in the Go standard library (with mutex locks).
* Less is more, keep the code concise and embody the essence of network programming

## Development Roadmap

- [x] Async write (refer example/async_http.go)
- [x] Websocket example
- [ ] Service oriented model
- [ ] Goev runtime GC zero pressure

## Contributing
Contributions are welcome! If you find any bugs or have suggestions for improvement, please open an issue or submit a pull request

## License
`goev` source code is available under the MIT License.
