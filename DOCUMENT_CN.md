# goev

框架核心就是OS的ev poll(epoll)机制

每个evpoll 可以认为是一个独立的执行栈, 它是线程安全的, 凡是绑定到evpoll中的链接, 在该执行栈中的I/O操作都是线程安全的  

鼓励开发者在evpoll执行栈中进行I/O操作, 解析到具体的业务数据后, 可以push到其他线程或协程异步处理, 但是跟I/O相关的, 最好是在evpoll执行栈中操作(也就是OnRead/OnWrite中)

**Reactor 是对evpoll的简单的管理对象, 对外提供统一接口**  
使用方法:  
```go
    reactor, err := goev.NewReactor(
        goev.EvDataArrSize(0),     // evpoll内部数据结构需要, 可以实际需要微调提升性能
        goev.EvReadyNum(512),      // evpoll(epoll)单次轮询处理的事件个数, 适当调整可以批量处理能力
        goev.EvPollNum(evPollNum), // 指定有多少个线程/协程来执行evpoll工作,
                                   // 每个evpoll独立绑定一个线程/协程, 所以不存在单个链接不存在并发处理
        goev.NoTimer(true),        // 如果没有定时器或connector的使用需求, 可以设置此参数,
                                   // 可以稍微减少一些不必要的开销

        goev.SetIOReadWriter(goev.NewIOReadWriter(32*1024, 1024*1024)),
                                   // 框架提供的默认I/O操作接口
    )
    if err != nil {
        panic(err.Error())
    }
```
Reactor通过对fd % evPollNum 均匀分布到各evpoll中去.  


**goev 是OOP的设计思想, 将每个链接视为一个对象(实现EvHandler接口), 链接有两种来源**  

**一种是acceptor接收到的**  
acceptor会指定一个 NewEvHandler的方法, 这样每接受到一个新链接, 就会通过此方法分配一个新的对象  
acceptor的使用方法:  
```go
    _, err = goev.NewAcceptor(
        forAcceptReactor,  // 第1个参数负责轮询listener fd的I/O事件
        forNewFdReactor,   // 第2个参数负责轮询listener fd接受到的新链接的I/O事件
                           // 前2个参数 也可以是同一值, 即所有I/O事件混在一个Reactor中处理
        func() goev.EvHandler { return new(Http) }, // 负责给新链接分配对象
        ":8080",           // 通常服务监听的地址格式 ipv4
        goev.ListenBacklog(256), // 后边是可选参数
        goev.ReuseAddr(true), // 一般服务器的标配参数(避免在TIME_WAIT状态下)
        goev.ReusePort(true), // 可以多进程/线程同时监听一个地址(ip:port)
        goev.SockRcvBufSize(16*1024), // 此参数是必须在listen 之前设定的, 对控制socket缓冲区的内存有帮助
    )

```

**一种是connector主动连接的**  
connector.Connect 方法会指定一个链接对象, 连接成功后此对象就会绑定新的链接  
connector的使用方法:  
```go
    c, err := NewConnector(
        r, // 非阻塞链接使用到的Reactor
        SockRcvBufSize(8*1024) // 新链接指定的接收缓冲区参数, 必须在connect之前设置
    )
    if err != nil {
        panic(err.Error())
    }
    err = c.Connect("127.0.0.1:9999", &Conn{}, 1000)
```

通过Reactor/Acceptor/Connector就可以组合出一个完全非阻塞/异步化的网络事件处理框架  

具体的网络事件处理逻辑在EvHandler的接口实现  
开发者要继承(内嵌)Event对象  
```go
type Http struct {
    goev.Event  // 特别注意: 如果想使用sync.pool之类的技术复用Http对象, 要记得调用Event.Init()
}
// 根据自己的需要, 实现具体的I/O事件处理方法
// OnOpen 是当acceptor/connector得到链接后首先调用的方法
func (h *Http) OnOpen(fd int, now int64) bool {
    // 新链接要手动指定要轮询的事件类型
    //
    // 特别注意: 当acceptor/connector使用的Reactor指定了多个EvPollNum时, 这时会出现线程切换,
    // 所以一些针对I/O操作的初始化过程要在AddEvHandler之前完成
    //
    // 比如我们要初始化一个buff, 那么如果该buff是在 AddEvHandler 之后初始化的, 
    // 很有可能OnRead方法在buff还没初始化完成就已经调用了
    if err := h.GetReactor().AddEvHandler(h, fd, goev.EvIn); err != nil {
        return false // 返回false 会直接回调OnClose方法
    }
    return true
}
// 处理可读事件(即: 链接有数据可以接收)
func (h *Http) OnRead(fd int, nio goev.IOReadWriter, now int64) bool {

    // goev.IOReadWriter 是框架内置的I/O操作方法, 使用一个全局的buf, 单个evpoll内所有链接共享,
    // 这样减少了临时内存分配和二次拷贝, 更对cpu cache友好!
    recvedData, err := nio.InitRead().Read(fd)
    if err == goev.ErrRcvBufOutOfLimit { // Abnormal connection
        return false
    }

    // handle recvedData
    // ...
    // build response

    // 构建响应数据, 同样是使用goev.IOReadWriter 内置的共享buf, 进行数据拼装, 减少临时内存分配和二次拷贝
    nio.InitWrite().Append(httpRespHeader).
        Append([]byte(liveDate.Load().(string))).
        Append(httpRespContentLength).
        Write(fd)
    return true
}
func (h *Http) OnClose(fd int) {
    // 释放资源, Http对象也会被gc回收的(前提是开发都没有单独将Http对象另外保存起来)
    netfd.Close(fd)
}
```


**定时器的使用**  
> **Golang已经内置Timer了, 为什么框架还要在引入?**  
  因为框架中引入timer能让I/O和timer事件全部绑定在一个线程/协栈执行栈中, 避免竞争
  一旦使用golang全局timer, 那么timer中如果有I/O操作, 所有I/O操作必要考虑并发保护, 这是一个非常大的开销
  

比如我们需要定时向对端发送Ping/Pong  
```go
func (h *Http) OnOpen(fd int, now int64) bool {
    if err := h.GetReactor().AddEvHandler(h, fd, goev.EvIn); err != nil {
        return false // 返回false 会直接回调OnClose方法
    }

    // 特别注意:
    // 1. 注册定时器是线程安全的, 内部有mutex保护
    // 2. 注册定时器一定要在 注册I/O事件之后, 这样才能确保跟I/O事件绑定到同一个evpoll上, 以保证Http对象的
    //    I/O和Timer事件都是并发安全的
    if err := h.GetReactor().SchedueTimer(h, 1000, 20*1000); err != nil { 
        h.GetReactor().RemoveEvHandler(eh, fd) // 要将刚才添加成功的操作, 撤销掉
        return false // 返回false 会直接回调OnClose方法
    }


    // 至此 Http对象 已经有两处引用: 1. evpoll中, 2. timer中
    return true
}
// 1秒后触发, 并以20秒为周期循环触发
func (h *Http) OnTimeout(now int64) bool {
    // 特别注意: 此时I/O事件可能已经触发过, 链接可能已经关闭, evpoll中不再持有Http对象
    if h.closed == true { // closed 变量不需要保护
        return false // end, timer removed
    }
    // send ping 这里目前并没有实现共享buf，做为下一步优化点
    return true  // keep interval timer
}
func (h *Http) OnClose(fd int) {
    // 释放资源, Http对象也会被gc回收的(前提是开发都没有单独将Http对象另外保存起来)
    netfd.Close(fd)
    h.closed = true
    h.GetReactor().CancelTimer(h)
}
```


至此, 一个完整的基础框架就完成了,
