## goev

`goev` is a high-performance, lightweight, i/o event-driven network framework in Go.

> Design Patterns Reference [ACE](http://www.dre.vanderbilt.edu/~schmidt/ACE-overview.html)

## 日志
- 2023-07-09 已经提交给TechEmpower, 测评代码[https://github.com/shaovie/FrameworkBenchmarks/tree/master/frameworks/Go/goev]
  顺便跑了一下gnet的测试程序, 同样的测试条件下, QPS goev胜过gnet 2.5%左右(2c4g的云主机环境, 仅做参考)

## Features

- 模型抽象简单，reactor/acceptor/connector/notify等
- 性能超级NB，没有任何多余的性能损耗


#### 关于EventPoll
##### 线程模型
- 开始选择L/F模型, 但后来发现当有水平触发模式下会有问题(如果事件处理不及时, 新的Leader会又捕捉到)
- 现在的模式就是每个evpoll单独一个线程, acceptor/connector/reactor可以随便组合,将事件注册到不同的evpoll中

##### 事件分发
- event poll捕捉到事件后，调用EvHandler接口，实现面向接口的业务处理

- 如果你的业务处理比较快, 那么直接在OnRead/OnWrite中处理业务是个不错的选择

- 如果你的业务处理比较慢, 那你应该把Reactor当做一个事件派发器, 在OnRead中异步处理业务, 这样保证epoll能高效运行

##### Acceptor
- 可以让你更优雅的创建 Listen service, 
- 它本质上就是实现了EvHandler的接口，处理listen socket的可读事件，然后将新接收到的fd注册到Reactor.evpoll中。
- 独立的接口可以让多个reactor开成pipeline式的处理

##### 关于Timer
- 目前还是简单的min heap实现, 但胜在简单稳定. 当单个evpoll的定时器到了万个的规模 就会有影响了. wheel还没搞完, 抽空再研究

##### 关于Log
- 使用syscal.Write减少了一次mutex.Lock(internal/poll/fd_unix.go:Fd.Write中有writeLock)
- 使用固定长度的[]byte格式化(fmt.Appendf)日志内容, 减少内存拷贝, 效率高(header中日期一天只需要格式化一次)
- 代码简单, 透明

##### 惯用写法
- 包含EINTR的场景一定要循环操作
- struct成员, 尽量按照字节从小到大依次排列, 根据字节对齐的原则, 会节省一些内存
- 小数据量(十几个左右)索引的时候, slice可能比map效率更高, 因为它的内存是连续的, 对cpu cache更友好, map是松散的 而且是动态创建, 内部涉及到的小对象更多

##### 一些零散的优化点
- ArrayMapUnion 联合索引结构
  是适合用int做为key索引的数组结构，index < arraySize 就用array存取，index >= arraySize 就用sync.Map存取，内存和索引速度可以兼得。
  
  原理：fd有个特点就是从0开始自增的int，当旧的释放后还会复用，我们就可以用fd做为索引定位保存的handler，当fd很大时，就用map索引
  
  进一步优化：经过测试整个array用一把锁，性能是很差的（在真正多线程环境下参考test/mutex_arr_vs_map.go），那么我们就可用atomic保存handler，创建一个[int]*atomic.Pointer[T]的数组，这样就大大减少了碰撞机会了，经过测试,可以比sync.Map快42%
  
- File/Socket 的读写操作尽量使用syscall以减少锁的使用, 标准库中封装都带Mutex(更合适全局空间使用)

- evpoll中提供一个全局的共享内存, 供各connection处理接收消息, 对CPU Cache非常友好, 能大大提升消息接收/处理过程的性能

#### 疑问
关于https://blog.51cto.com/u_15087084/2597531 中讲到的msec, 调整msec是对下次事件轮询的预判, 主动让出CPU不就延缓了下次轮询的时机吗?
我觉得它可能只是单纯测试了epoll_wait的执行时间, 并没有实际放fd进去

#### Bugs
- 整理成文章了 [使用syscall.Epoll_* 关联内存被GC释放导致的崩溃](https://zhuanlan.zhihu.com/p/640712548)
