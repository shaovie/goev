# goev

`goev` is a lightweight, concise i/o event demultiplexer implementation in Go.

> Design Patterns Reference[ACE](http://www.dre.vanderbilt.edu/~schmidt/ACE-overview.html)

# Features

- 

linux kernel version >= 2.6.28
    for `man 2 accept4`



- Reactor
    基于Leader/Follower 模型, Leader负责epoll_wait, 当获取到I/O事件后, 转为Follower,
    释放互斥锁并产生一个新的Leader(根据Mutex的唤醒顺序), Follower负责处理刚刚获取到的I/O事件(在这里不会出现数据交换), 当事件处理完后又回重新排队等待提升为Leader
    L/F 最大程度实现并发处理I/O事件, 减少线程切换, 消除线程间的数据切换和不必要的数据拷贝

    如果你的业务处理比较快, 那么直接在OnRead/OnWrite中处理业务是个不错的选择

    如果你的业务处理比较慢, 那你应该把Reactor当做一个事件派发器, 在OnRead中异步处理业务, 这样保证L/F能高效运行

- Acceptor
    可以让你更优雅的创建 Listen service, 

evpool
优化 TODO
    1. https://blog.51cto.com/u_15087084/2597531
       疑问, 调整msec是对下次事件轮询的预判, 主动让出CPU不就延缓了下次轮询的时机吗?
       我觉得它可能只是单纯测试了epoll_wait的执行时间, 并没有实际放fd进去

