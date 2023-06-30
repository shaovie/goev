## goev

`goev` is a lightweight, concise i/o event demultiplexer implementation in Go.

> Design Patterns Reference [ACE](http://www.dre.vanderbilt.edu/~schmidt/ACE-overview.html)

## Features

- 模型抽象简单，reactor/acceptor/connector/notify等
- 性能超级NB，没有任何多余的性能损耗


#### 关于EventPoll
##### 线程模型
- 起初随机选中的Leader(线程)负责epoll_wait, 当获取到I/O事件后转为Follower(不会出现数据拷贝和上下文切换),就地处理刚拿到I/O事件。
- 同时释放Leader锁并产生一个新的Leader(根据Mutex的唤醒顺序), Follower把事件处理完后又回重新排队等待提升为Leader

- L/F 模型能最大程度实现并发处理I/O事件, 减少线程切换, 消除线程间的数据切换和不必要的数据拷贝

##### 事件分发
- event poll捕捉到事件后，调用EvHandler接口，实现面向接口的业务处理

- 如果你的业务处理比较快, 那么直接在OnRead/OnWrite中处理业务是个不错的选择

- 如果你的业务处理比较慢, 那你应该把Reactor当做一个事件派发器, 在OnRead中异步处理业务, 这样保证L/F能高效运行

##### Acceptor
- 可以让你更优雅的创建 Listen service, 
- 它本质上就是实现了EvHandler的接口，处理listen socket的可读事件，然后将新接收到的fd注册到Reactor.evpoll中。

##### 一些零散的优化点
- ArrayMapUnion 联合索引结构
  是适合用int做为key索引的数组结构，index < arraySize 就用array存取，index >= arraySize 就用sync.Map存取，内存和索引速度可以兼得。
  
  原理：fd有个特点就是从0开始自增的int，当旧的释放后还会复用，我们就可以用fd做为索引定位保存的handler，当fd很大时，就用map索引
  
  进一步优化：经过测试整个array用一把锁，性能是很差的（在真正多线程环境下参考test/mutex_arr_vs_map.go），那么我们就可用atomic保存handler，创建一个[int]*atomic.Pointer[T]的数组，这样就大大减少了碰撞机会了，经过测试,可以比sync.Map快42%
  
#### 疑问
关于https://blog.51cto.com/u_15087084/2597531 中讲到的msec, 调整msec是对下次事件轮询的预判, 主动让出CPU不就延缓了下次轮询的时机吗?
我觉得它可能只是单纯测试了epoll_wait的执行时间, 并没有实际放fd进去

#### Bugs
- 整理成文章了 [使用syscall.Epoll_* 关联内存被GC释放导致的崩溃](https://zhuanlan.zhihu.com/p/640712548)
 
### 笔记      
- linux kernel version >= 2.6.28
    for `man 2 accept4`
